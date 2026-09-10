package kernel_test

import (
	"strings"
	"testing"

	"github.com/semirm-dev/ghu/internal/kernel"
)

// Keys live in ~/.ssh on every platform ghu ships for: OpenSSH on Windows uses
// %USERPROFILE%\.ssh, and Git for Windows uses ~/.ssh, so one rule covers all
// three.
//
// The homes below are already forward-slashed, which is what Expand sees: it
// runs the real home through filepath.ToSlash first, and on Windows that turns
// C:\Users\you into C:/Users/you. Forward slashes are what git wants inside
// config values on every platform, and what OpenSSH accepts for -i.
func TestKeyPathIsTheSameEverywhere(t *testing.T) {
	homes := map[string]string{
		"linux":   "/home/you",
		"macos":   "/Users/you",
		"windows": "C:/Users/you",
	}

	// The three ways someone might write the same key.
	written := []string{`private`, `~/.ssh/private`, `~\.ssh\private`}

	for osName, home := range homes {
		want := home + "/.ssh/private"

		for _, in := range written {
			p := kernel.Profile{Name: "w", Dir: "~/code", User: "u", Email: "u@x.com", Key: in}
			if err := p.Validate(); err != nil {
				t.Fatalf("%s: %q was rejected: %v", osName, in, err)
			}

			got := kernel.Expand(p.KeyPath(), home)
			if got != want {
				t.Errorf("%s: %q resolved to %q, want %q", osName, in, got, want)
			}
			if cmd := kernel.SSHCommand(got); strings.ContainsRune(cmd, '\\') {
				t.Errorf("%s: %q produced a backslash in core.sshCommand: %s", osName, in, cmd)
			}
		}
	}
}

// A key outside ~/.ssh is refused rather than quietly accepted. A relative one
// resolves against the caller's working directory, so it works in one shell and
// not the next -- and doctor would stat the same relative path and call it fine.
func TestKeyOutsideSSHDirIsRefused(t *testing.T) {
	for _, key := range []string{
		"~/keys/id_work",
		"/etc/ssh/id_work",
		"./id_work",
		`C:\keys\id_work`,
		"../id_work",
	} {
		p := kernel.Profile{Name: "w", Dir: "~/code", User: "u", Email: "u@x.com", Key: key}

		err := p.Validate()
		if err == nil {
			t.Errorf("%q was accepted, expected a refusal", key)
			continue
		}
		if !strings.Contains(err.Error(), kernel.KeyDir) {
			t.Errorf("%q: error does not say where keys belong: %v", key, err)
		}
	}
}
