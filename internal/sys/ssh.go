// SSH is the adapter for ssh key generation and the GitHub identity probe.
//
// ghu creates keys but never reads or writes ~/.ssh/config, and deletes one
// only when --force says to replace it.

package sys

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const KeyMode os.FileMode = 0o600

const PublicKeyMode os.FileMode = 0o644

// PublicKeySuffix reaches the public half of a key.
const PublicKeySuffix = ".pub"

// greeting matches GitHub's reply to an authenticated ssh session.
var greeting = regexp.MustCompile(`Hi ([A-Za-z0-9-]+)!`)

type SSH struct {
	run *Runner
}

// GenerateOpts describes one keypair to create.
type GenerateOpts struct {
	Path    string
	Comment string

	// Force permits replacing an existing key.
	Force bool
}

// GenerateResult reports what Generate did, or would have done.
type GenerateResult struct {
	// Replaced is true when an existing key was overwritten under Force.
	Replaced bool
}

type KeyProblem struct {
	Path    string
	Message string
}

func NewSSH(r *Runner) *SSH { return &SSH{run: r} }

func PublicKey(path string) (string, error) {
	data, err := os.ReadFile(path + PublicKeySuffix)
	if err != nil {
		return "", fmt.Errorf("reading %s.pub: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// CheckKey verifies that both halves of a key exist with sane permissions. It
// never reads or logs private key material.
func CheckKey(path string) []KeyProblem {
	var problems []KeyProblem

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return []KeyProblem{{path, "private key is missing"}}
	}
	if err != nil {
		return []KeyProblem{{path, "cannot stat: " + err.Error()}}
	}

	if mode := info.Mode().Perm(); mode != KeyMode {
		problems = append(problems, KeyProblem{
			path, fmt.Sprintf("permissions are %04o, want %04o", mode, KeyMode),
		})
	}

	if _, err := os.Stat(path + PublicKeySuffix); os.IsNotExist(err) {
		problems = append(problems, KeyProblem{path + PublicKeySuffix, "public key is missing"})
	}
	return problems
}

// DryRun reports whether this SSH suppresses its writes.
func (c *SSH) DryRun() bool { return c.run.DryRun() }

// Generate creates an ed25519 keypair. It refuses to overwrite an existing key
// unless Force is set: a lost private key means losing access to every host
// that trusts it.
func (c *SSH) Generate(ctx context.Context, opts GenerateOpts) (GenerateResult, error) {
	var result GenerateResult

	// Generate changes the filesystem directly as well as through the runner:
	// it deletes the old key before ssh-keygen replaces it. Under a dry run the
	// runner would swallow the replacement, so skipping the deletion here is
	// what stops --force --dry-run from destroying a key it never rewrites.
	dry := c.DryRun()

	if _, err := os.Stat(opts.Path); err == nil {
		if !opts.Force {
			return result, fmt.Errorf("%s already exists; pass --force to overwrite it", opts.Path)
		}
		result.Replaced = true

		if !dry {
			if err := os.Remove(opts.Path); err != nil {
				return result, fmt.Errorf("removing %s: %w", opts.Path, err)
			}
			// ssh-keygen refuses to write when only the public half survives.
			os.Remove(opts.Path + PublicKeySuffix)
		}
	}

	if !dry {
		dir := filepath.Dir(opts.Path)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return result, fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	out, err := c.run.Run(ctx, "ssh-keygen",
		"-t", "ed25519",
		"-f", opts.Path,
		"-C", opts.Comment,
		"-N", "",
	)
	if err != nil {
		return result, fmt.Errorf("ssh-keygen: %w: %s", err, strings.TrimSpace(out))
	}
	return result, nil
}

// Probe authenticates to GitHub with one specific key and reports which
// account answers -- the only check that proves a key belongs to the account a
// profile claims. GitHub always closes the session with exit status 1, so the
// exit code is deliberately ignored and only the greeting is read.
func (c *SSH) Probe(ctx context.Context, keyPath string) (string, error) {
	out, _ := c.run.Query(ctx, "ssh",
		"-i", keyPath,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-T", "git@github.com",
	)

	if m := greeting.FindStringSubmatch(out); m != nil {
		return m[1], nil
	}

	msg := strings.TrimSpace(out)
	if msg == "" {
		msg = "no response from github.com"
	}
	return "", fmt.Errorf("%s", firstLine(msg))
}

func (p KeyProblem) Error() string { return p.Path + ": " + p.Message }

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
