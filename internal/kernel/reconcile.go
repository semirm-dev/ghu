package kernel

// Reconciling ~/.ghu and ~/.gitconfig to match a config: which files get
// generated, which stale ones go, and which includeIf entries ghu owns.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/semirm-dev/ghu/internal/kernel/sys"
)

type ApplyResult struct {
	Backup          BackupResult `json:"backup"`
	ProfilesWritten []string     `json:"profiles_written"`
	ProfilesRemoved []string     `json:"profiles_removed"`
	IncludesRemoved []string     `json:"includes_removed"`
	IncludesWritten []string     `json:"includes_written"`
}

// Reconciler writes what a config implies: one generated git config file per
// profile, and one includeIf entry per profile in ~/.gitconfig.
//
// It holds the two collaborators it needs and nothing else. The config is an
// argument rather than a field: reconciling is a function of the config handed
// to it, and a stored copy would be one more thing to keep in step.
type Reconciler struct {
	Layout Layout
	Git    *sys.Git
}

// DryRun reports whether this Reconciler's writes are suppressed. It is read
// from the runner behind Git, never stored, so there is one answer.
func (r Reconciler) DryRun() bool { return r.Git.DryRun() }

// Apply reconciles ~/.ghu and ~/.gitconfig to match cfg.
//
// Idempotent by construction: generated files are rewritten from scratch and
// ghu's includeIf entries are removed before being re-added, so an edited
// profile takes effect without leaving stale keys behind.
func (r Reconciler) Apply(ctx context.Context, cfg Config) (ApplyResult, error) {
	var result ApplyResult

	if err := cfg.Validate(); err != nil {
		return result, err
	}

	dry := r.Git.DryRun()

	if !dry {
		if err := r.Layout.EnsureDirs(); err != nil {
			return result, err
		}

		// ~/.gitconfig is the only file ghu touches that the user did not ask
		// it to create. --dry-run must leave the filesystem exactly as it
		// found it.
		backup, err := r.Layout.BackupGitConfig(time.Now())
		if err != nil {
			return result, err
		}
		result.Backup = backup
	}

	written, err := r.writeProfileFiles(ctx, cfg)
	if err != nil {
		return result, err
	}
	result.ProfilesWritten = written

	removed, err := r.removeOrphanFiles(cfg)
	if err != nil {
		return result, err
	}
	result.ProfilesRemoved = removed

	unset, err := r.clearIncludes(ctx)
	if err != nil {
		return result, err
	}
	result.IncludesRemoved = unset

	added, err := r.writeIncludes(ctx, cfg)
	if err != nil {
		return result, err
	}
	result.IncludesWritten = added

	return result, nil
}

// writeProfileFiles regenerates every profile's git config file.
//
// Deleted first and rebuilt rather than patched: a profile that stops signing
// must lose commit.gpgsign, and unsetting keys that may or may not be there is
// more failure modes than starting from nothing.
func (r Reconciler) writeProfileFiles(ctx context.Context, cfg Config) ([]string, error) {
	var written []string
	dry := r.Git.DryRun()

	for _, p := range cfg.Profiles {
		path := r.Layout.ProfileFile(p.Name)

		if !dry {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return written, fmt.Errorf("removing %s: %w", path, err)
			}
		}

		for _, entry := range Entries(p, r.Layout.Home) {
			if err := r.Git.Set(ctx, sys.File(path), entry.Key, entry.Value); err != nil {
				return written, fmt.Errorf("writing profile %q: %w", p.Name, err)
			}
		}

		if !dry {
			// git config creates the file 0644; these name an identity.
			if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
				return written, fmt.Errorf("setting mode on %s: %w", path, err)
			}
		}

		written = append(written, p.Name)
	}
	return written, nil
}

// removeOrphanFiles deletes generated files for profiles that no longer exist.
// Anything without ghu's .gitconfig naming is left alone.
func (r Reconciler) removeOrphanFiles(cfg Config) ([]string, error) {
	entries, err := os.ReadDir(r.Layout.ProfilesDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", r.Layout.ProfilesDir(), err)
	}

	live := map[string]bool{}
	for _, p := range cfg.Profiles {
		live[p.Name] = true
	}

	var removed []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ProfileFileSuffix)
		if e.IsDir() || name == e.Name() || live[name] {
			continue
		}

		if !r.Git.DryRun() {
			path := filepath.Join(r.Layout.ProfilesDir(), e.Name())
			if err := os.Remove(path); err != nil {
				return removed, fmt.Errorf("removing %s: %w", path, err)
			}
		}
		removed = append(removed, name)
	}
	return removed, nil
}

// clearIncludes unsets every includeIf entry ghu owns.
//
// Ownership is decided by where the entry points, not by its key: an entry not
// pointing inside ~/.ghu/profiles was written by the user and is left alone.
func (r Reconciler) clearIncludes(ctx context.Context) ([]string, error) {
	existing, err := r.Git.GetRegexp(ctx, sys.Global(), `^includeIf\.`)
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, kv := range existing {
		if !r.Layout.OwnsProfilePath(kv.Value) {
			continue
		}
		if err := r.Git.Unset(ctx, sys.Global(), kv.Key); err != nil {
			return removed, err
		}
		removed = append(removed, kv.Key)
	}
	return removed, nil
}

// writeIncludes adds ghu's includeIf entries shortest directory first.
//
// Order is the whole point: git applies includeIf in file order and the last
// match wins, so a nested profile only beats its parent when the parent was
// written first. clearIncludes has just emptied ghu's entries, so these append
// in exactly this order.
func (r Reconciler) writeIncludes(ctx context.Context, cfg Config) ([]string, error) {
	includes := Includes(cfg, r.Layout.Home, r.Layout.ProfileFile)

	var written []string
	for _, inc := range includes {
		if err := r.Git.Set(ctx, sys.Global(), inc.Key, inc.Path); err != nil {
			return written, fmt.Errorf("writing includeIf for %q: %w", inc.Profile, err)
		}
		written = append(written, inc.Key)
	}
	return written, nil
}
