// Package kernel is what more than one ghu feature needs: the profile model,
// the ~/.ghu layout, directory resolution, the git config a profile implies,
// and the reconciler that writes it.
//
// It earns each piece. Profile and Config are the aggregate every slice reads.
// Layout is where ghu keeps its files. Expand, Match and Ordered decide which
// profile governs a directory, the same way git's gitdir patterns do. Entries,
// Includes and IncludeKey are the contract between the reconciler that writes a
// profile's git config and doctor that verifies it -- two features that must
// agree on the same shape without importing each other.
//
// What one feature alone uses is not here: loading and saving ~/.ghu/config.yaml
// lives in profiles, because nothing else touches it.
//
// Nothing here renders, reads a flag, or prompts. Operations return values --
// already JSON-tagged, because those structs are the API -- and presentation
// lives in the adapters, so every one of them drives the same code.
//
// Dry-run state is stored nowhere. Reconciler asks the Git it holds, which asks
// the one sys.Runner behind it.
package kernel

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/semirm-dev/ghu/internal/kernel/sys"
)

// The domain model: a profile, and the config that holds a set of them.

const ConfigVersion = 2

// KeyDir is where ssh keys live, and the only place a profile may point.
//
// ssh's own defaults assume it, every tool that manages keys writes there, and
// allowing anywhere else buys a class of silent breakage: a relative path
// resolves against whatever directory you happened to run from, so a key that
// looks fine in the config works in one shell and not the next.
const KeyDir = "~/.ssh"

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Profile is one GitHub identity bound to a directory tree.
type Profile struct {
	Name string `yaml:"name"`
	Dir  string `yaml:"dir"`
	// User is the name on your commits -- git's user.name. It is a display
	// name, free text, and not your GitHub account.
	User string `yaml:"user"`

	// Login is your GitHub account name, the one in github.com/<login>. Git
	// never sees it; `ghu doctor` uses it to check the key authenticates as
	// the account this profile claims. Empty means doctor reports which
	// account answered without judging it.
	Login string `yaml:"login,omitempty"`
	Email string `yaml:"email"`
	// Key is the private key this profile pushes with. It always lives in
	// ~/.ssh: a bare name is taken as a file there, and anywhere else is
	// refused. See KeyDir.
	Key  string `yaml:"key"`
	Sign bool   `yaml:"sign"`
}

type Config struct {
	Version  int       `yaml:"version"`
	Profiles []Profile `yaml:"profiles"`
}

// Resolving which profile governs a directory, and reporting the set as rows.
//
// Nothing here keeps state, so nothing here is a method: these are functions of
// a config and a home directory.

// MatchCurrent returns the profile governing the working directory, trying the
// configured paths first and then again with symlinks resolved on both sides.
// sys.CurrentDir already returns a resolved path, so a profile whose directory
// passes through a symlink -- /var, a link to /private/var on macOS, is the
// common one -- would otherwise never match its own repositories.
func MatchCurrent(cfg Config, home string) (Profile, bool) {
	dir, err := sys.CurrentDir()
	if err != nil {
		return Profile{}, false
	}

	if p, ok := Match(cfg.Profiles, dir, home); ok {
		return p, true
	}
	return Match(resolved(cfg.Profiles, home), dir, home)
}

// UnknownProfile is the error every operation returns for a name that is not
// configured. Listing what is configured turns a typo into a one-line fix, and
// an empty config into a pointer at `ghu init`.
func UnknownProfile(cfg Config, name string) error {
	names := cfg.Names()
	if len(names) == 0 {
		return fmt.Errorf("unknown profile %q: no profiles are configured, run `ghu init` first", name)
	}
	return fmt.Errorf("unknown profile %q: configured profiles are %s", name, strings.Join(names, ", "))
}

// NormalizeKey expands a bare key name into ~/.ssh, and settles on forward
// slashes. Anything already carrying a separator is left for Validate to
// accept or refuse.
//
// Both separators are treated as separators on every platform, not just the
// native one: ~/.ghu/config.yaml is a portable file, and a key written on
// Windows as ~\.ssh\id_work has to mean the same thing when the same config is
// read on Linux.
func NormalizeKey(key string) string {
	key = keySlash(strings.TrimSpace(key))
	if key == "" || strings.ContainsRune(key, '/') {
		return key
	}
	return KeyDir + "/" + key
}

// ClaimsLogin reports whether this profile says which GitHub account its key
// belongs to, which is what makes doctor's probe a check rather than a report.
func (p Profile) ClaimsLogin() bool { return strings.TrimSpace(p.Login) != "" }

// KeyPath is the profile's key as ghu stores it: a bare name means a file in
// ~/.ssh, which is how you would say it out loud.
func (p Profile) KeyPath() string { return NormalizeKey(p.Key) }

func (p Profile) Validate() error {
	if !namePattern.MatchString(p.Name) {
		return fmt.Errorf("profile name %q must match %s", p.Name, namePattern)
	}
	for _, f := range []struct{ label, value string }{
		{"dir", p.Dir},
		{"user", p.User},
		{"email", p.Email},
		{"key", p.KeyPath()},
	} {
		if strings.TrimSpace(f.value) == "" {
			return fmt.Errorf("profile %q: %s is required", p.Name, f.label)
		}
	}
	if !strings.Contains(p.Email, "@") {
		return fmt.Errorf("profile %q: email %q is not an address", p.Name, p.Email)
	}

	// A key outside ~/.ssh is refused rather than quietly accepted: an
	// absolute path elsewhere is unusual, and a relative one resolves against
	// the caller's working directory, which is how a profile ends up working
	// in one shell and failing in the next.
	if key := NormalizeKey(p.Key); !strings.HasPrefix(key, KeyDir+"/") {
		return fmt.Errorf("profile %q: key %q must be in %s -- write it as %s/%s",
			p.Name, p.Key, KeyDir, KeyDir, path.Base(key))
	}

	// ghu derives the public half by appending .pub, so a key that already
	// ends in .pub yields nonsense like id_x.pub.pub and reports confusing
	// problems against a file that was never meant to exist.
	if strings.HasSuffix(p.Key, sys.PublicKeySuffix) {
		return fmt.Errorf("profile %q: key %q is a public key; point key at the private half, %q",
			p.Name, p.Key, strings.TrimSuffix(p.Key, sys.PublicKeySuffix))
	}
	return nil
}

// Duplicate names would collide on disk; duplicate directories would make
// matching ambiguous.
func (c Config) Validate() error {
	seenName := map[string]bool{}
	seenDir := map[string]bool{}
	for _, p := range c.Profiles {
		if err := p.Validate(); err != nil {
			return err
		}
		if seenName[p.Name] {
			return fmt.Errorf("duplicate profile name %q", p.Name)
		}
		seenName[p.Name] = true

		dir := path.Clean(p.Dir)
		if seenDir[dir] {
			return fmt.Errorf("duplicate profile directory %q", p.Dir)
		}
		seenDir[dir] = true
	}
	return nil
}

func (c Config) Find(name string) (Profile, bool) {
	for _, p := range c.Profiles {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Profile{}, false
}

// Names lists profile names in declaration order.
func (c Config) Names() []string {
	names := make([]string, 0, len(c.Profiles))
	for _, p := range c.Profiles {
		names = append(names, p.Name)
	}
	return names
}

// keySlash normalises both separators to /, whatever the host uses.
func keySlash(p string) string { return strings.ReplaceAll(slash(p), `\`, "/") }

// resolved copies the profiles with each directory passed through
// EvalSymlinks. A directory that cannot be resolved -- because it does not
// exist -- is left as configured, so the copy is always usable.
func resolved(profiles []Profile, home string) []Profile {
	out := make([]Profile, 0, len(profiles))

	for _, p := range profiles {
		abs := Expand(p.Dir, home)
		if link, err := filepath.EvalSymlinks(abs); err == nil {
			p.Dir = link
		} else {
			p.Dir = abs
		}
		out = append(out, p)
	}
	return out
}
