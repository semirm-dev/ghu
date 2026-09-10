package kernel

// The ~/.ghu directory: where the config, the generated profile files and the
// backups live, and how ghu preserves the one file it did not create,
// ~/.gitconfig.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/semirm-dev/ghu/internal/kernel/sys"
)

const BackupsKept = 10

// snapshotStamp is the layout snapshotName matches; the two must agree.
const snapshotStamp = "20060102T150405Z"

// ProfileFileSuffix names the generated per-profile git config files. It is
// also what identifies one when reading the directory back, or when reading an
// includeIf value, so it is declared once.
const ProfileFileSuffix = ".gitconfig"

// PivotName is the first backup ghu ever takes: your ~/.gitconfig as it stood
// before ghu touched anything. It is never pruned and never rewritten.
const PivotName = "gitconfig.original"

// snapshotName matches the rolling, prunable snapshots by their full
// timestamped shape. Matching a bare "gitconfig.2" prefix would also sweep up
// any other file a user parked in the backups directory -- and stop working
// in the year 3000.
var snapshotName = regexp.MustCompile(`^gitconfig\.\d{8}T\d{6}Z$`)

type Layout struct {
	Home string
	Root string
}

// BackupResult describes what a backup run preserved.
type BackupResult struct {
	// Snapshot is the rolling copy taken this run, empty when none was.
	Snapshot string `json:"snapshot,omitempty"`
	// Pivot is the permanent original.
	Pivot string `json:"pivot"`
	// Established is true when this run created the pivot.
	Established bool `json:"established"`
}

// NewLayout builds the layout for a home directory. An empty home is looked up.
func NewLayout(home string) (Layout, error) {
	if home == "" {
		found, err := os.UserHomeDir()
		if err != nil {
			return Layout{}, fmt.Errorf("resolving home directory: %w", err)
		}
		home = found
	}
	return LayoutAt(home), nil
}

// LayoutAt builds a layout rooted at an explicit home.
func LayoutAt(home string) Layout {
	return Layout{Home: home, Root: filepath.Join(home, ".ghu")}
}

func (l Layout) ConfigFile() string { return filepath.Join(l.Root, "config.yaml") }

func (l Layout) ProfilesDir() string { return filepath.Join(l.Root, "profiles") }

func (l Layout) BackupsDir() string { return filepath.Join(l.Root, "backups") }

func (l Layout) ProfileFile(name string) string {
	return filepath.Join(l.ProfilesDir(), name+ProfileFileSuffix)
}

// GitConfig is ~/.gitconfig -- the only file ghu modifies that the user did
// not ask it to create, which is why it is backed up before every write.
func (l Layout) GitConfig() string { return filepath.Join(l.Home, ".gitconfig") }

// SSHDir is ~/.ssh. ghu creates keys here but never edits ~/.ssh/config.
func (l Layout) SSHDir() string { return filepath.Join(l.Home, ".ssh") }

func (l Layout) EnsureDirs() error {
	for _, dir := range []string{l.Root, l.ProfilesDir(), l.BackupsDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return nil
}

// OwnsProfilePath tells ghu's own includeIf entries apart from any the user
// wrote by hand, so that reconciling never removes someone else's.
func (l Layout) OwnsProfilePath(path string) bool {
	return strings.HasPrefix(filepath.Clean(path), l.ProfilesDir()+string(filepath.Separator))
}

// BackupGitConfig preserves ~/.gitconfig before ghu modifies it.
//
// The very first backup becomes the pivot: the file as it was before ghu ever
// ran, kept forever and excluded from pruning. Every later run adds a rolling
// snapshot, of which the most recent BackupsKept are retained. Without the
// pivot the one copy actually worth having -- the pristine original -- would
// eventually be evicted by routine reconciles.
func (l Layout) BackupGitConfig(now time.Time) (BackupResult, error) {
	pivot := filepath.Join(l.BackupsDir(), PivotName)
	result := BackupResult{Pivot: pivot}

	if err := os.MkdirAll(l.BackupsDir(), 0o700); err != nil {
		return result, fmt.Errorf("creating %s: %w", l.BackupsDir(), err)
	}

	data, err := os.ReadFile(l.GitConfig())
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		return result, fmt.Errorf("reading %s: %w", l.GitConfig(), err)
	}

	switch _, statErr := os.Stat(pivot); {
	case statErr == nil:
		// The pivot is write-once. Rewriting it would replace the original
		// with a copy ghu had already modified.

	case os.IsNotExist(statErr):
		// An absent ~/.gitconfig is preserved as an empty pivot rather than
		// skipped: git treats an empty global config the same as none, and
		// recording nothing here would let the next run enshrine a file ghu
		// itself wrote as the "original".
		if err := sys.WriteAtomic(pivot, data, 0o600); err != nil {
			return result, err
		}
		result.Established = true
		return result, nil

	default:
		return result, fmt.Errorf("reading %s: %w", pivot, statErr)
	}

	if missing {
		return result, nil
	}

	stamp := now.UTC().Format(snapshotStamp)
	dst := filepath.Join(l.BackupsDir(), "gitconfig."+stamp)

	if err := sys.WriteAtomic(dst, data, 0o600); err != nil {
		return result, err
	}
	result.Snapshot = dst

	return result, l.pruneBackups()
}

func (l Layout) pruneBackups() error {
	entries, err := os.ReadDir(l.BackupsDir())
	if err != nil {
		return fmt.Errorf("reading %s: %w", l.BackupsDir(), err)
	}

	// The pivot is deliberately excluded: it is not part of the rolling
	// window and must survive every prune.
	var names []string
	for _, e := range entries {
		if !e.IsDir() && snapshotName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	if len(names) <= BackupsKept {
		return nil
	}

	// Names are timestamped, so lexical order is chronological.
	sort.Strings(names)
	for _, name := range names[:len(names)-BackupsKept] {
		if err := os.Remove(filepath.Join(l.BackupsDir(), name)); err != nil {
			return fmt.Errorf("pruning backup %s: %w", name, err)
		}
	}
	return nil
}
