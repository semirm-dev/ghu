package core

// Turning a profile into the git config key/value pairs that represent it:
// which keys get set, in what order, with what values. No I/O happens here --
// git itself does the writing.

import (
	"fmt"

	"github.com/semirm-dev/ghu/internal/core/sys"
)

type Entry struct {
	Key   string
	Value string
}

// Include is one includeIf entry in ~/.gitconfig, e.g.
// includeIf.gitdir/i:/Users/you/code/.path pointing at a profile config file.
type Include struct {
	Key     string
	Path    string
	Profile string
}

// SSHCommand is the core.sshCommand value binding a key to a profile.
// IdentitiesOnly stops ssh-agent from offering a different key first, which is
// the usual cause of pushing as the wrong account.
func SSHCommand(keyPath string) string {
	return fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", keyPath)
}

// Entries lists every key/value pair in a profile's generated config file, in
// write order. Paths are absolute: git expands ~ in include.path and in
// includeIf patterns, but core.sshCommand only reaches a shell when it holds
// shell metacharacters, so a ~ there is not reliably expanded.
func Entries(p Profile, home string) []Entry {
	key := Expand(p.KeyPath(), home)

	entries := []Entry{
		{"user.name", p.User},
		{"user.email", p.Email},
		{"core.sshCommand", SSHCommand(key)},
	}

	if p.Sign {
		entries = append(entries,
			Entry{"user.signingkey", key + sys.PublicKeySuffix},
			Entry{"gpg.format", "ssh"},
			Entry{"commit.gpgsign", "true"},
		)
	}
	return entries
}

// IncludeKey builds the ~/.gitconfig key for a profile directory. gitdir/i is
// used unconditionally: macOS and Windows default to case-insensitive
// filesystems, where plain gitdir silently fails to match. The trailing slash
// is what makes the pattern match the tree recursively rather than the single
// directory.
func IncludeKey(dir, home string) string {
	abs := Expand(dir, home)
	if abs != "/" {
		abs += "/"
	}
	return fmt.Sprintf("includeIf.gitdir/i:%s.path", abs)
}

// Includes lists the includeIf entries for a config, shortest directory first
// so that the deepest matching tree wins in git's last-match-wins resolution.
func Includes(cfg Config, home string, profilePath func(name string) string) []Include {
	ordered := Ordered(cfg.Profiles, home)

	out := make([]Include, 0, len(ordered))
	for _, p := range ordered {
		out = append(out, Include{
			Key:     IncludeKey(p.Dir, home),
			Path:    profilePath(p.Name),
			Profile: p.Name,
		})
	}
	return out
}
