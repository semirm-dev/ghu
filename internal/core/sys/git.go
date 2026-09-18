// Git is the adapter over the git config command. Every write ghu makes to a
// git config file goes through here, so git owns quoting, section merging and
// idempotency; ghu never assembles git config syntax itself.

package sys

import (
	"context"
	"errors"
	"fmt"
	osexec "os/exec"
	"strings"
)

var ErrNotFound = errors.New("config key not set")

// Scope selects which config file a command applies to.
type Scope struct {
	flag string
	file string
}

type Git struct {
	runner *Runner
}

type KeyValue struct {
	Key   string
	Value string
}

type Origin struct {
	Key   string
	Value string
	File  string
}

func Global() Scope { return Scope{flag: "--global"} }

func Local() Scope { return Scope{flag: "--local"} }

func File(path string) Scope { return Scope{flag: "--file", file: path} }

func NewGit(r *Runner) *Git { return &Git{runner: r} }

// DryRun reports whether this Git suppresses its writes.
func (git *Git) DryRun() bool { return git.runner.DryRun() }

// Set replaces every value for a key with one. Plain `git config key value`
// errors if the key already holds more than one value; --replace-all is what
// keeps a repeated Set idempotent no matter how many values a prior write
// left behind.
func (git *Git) Set(ctx context.Context, s Scope, key, value string) error {
	args := append([]string{"config"}, s.args()...)
	args = append(args, "--replace-all", key, value)

	out, err := git.runner.Run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("git config %s: %w: %s", key, err, strings.TrimSpace(out))
	}
	return nil
}

// Unset removes every value for a key. A key that is already absent is not an
// error, so callers can unset unconditionally.
func (git *Git) Unset(ctx context.Context, s Scope, key string) error {
	args := append([]string{"config"}, s.args()...)
	args = append(args, "--unset-all", key)

	out, err := git.runner.Run(ctx, "git", args...)
	if err != nil {
		// Exit code 5 means the key was not there to begin with.
		if code(err) == 5 {
			return nil
		}
		return fmt.Errorf("git config --unset-all %s: %w: %s", key, err, strings.TrimSpace(out))
	}
	return nil
}

// Get reads a single value. git exits 1 when the key is not set, which becomes
// ErrNotFound.
func (git *Git) Get(ctx context.Context, s Scope, key string) (string, error) {
	args := append([]string{"config"}, s.args()...)
	args = append(args, "--get", key)

	out, err := git.runner.Query(ctx, "git", args...)
	if err != nil {
		if code(err) == 1 {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("git config --get %s: %w: %s", key, err, strings.TrimSpace(out))
	}
	return strings.TrimRight(out, "\n"), nil
}

// GetRegexp lists every key matching a pattern, in file order.
func (git *Git) GetRegexp(ctx context.Context, s Scope, pattern string) ([]KeyValue, error) {
	args := append([]string{"config"}, s.args()...)
	args = append(args, "--get-regexp", pattern)

	out, err := git.runner.Query(ctx, "git", args...)
	if err != nil {
		// Exit code 1 means nothing matched, which is an empty result.
		if code(err) == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("git config --get-regexp %s: %w: %s", pattern, err, strings.TrimSpace(out))
	}

	var found []KeyValue
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		found = append(found, KeyValue{Key: key, Value: value})
	}
	return found, nil
}

// ShowOrigin resolves a key the way git itself would from the current
// directory, and reports which file supplied the winning value. That is what
// makes `ghu profile show` report what git resolves rather than what ghu's config
// claims.
func (git *Git) ShowOrigin(ctx context.Context, key string) (Origin, error) {
	out, err := git.runner.Query(ctx, "git", "config", "--show-origin", "--get", key)
	if err != nil {
		if code(err) == 1 {
			return Origin{Key: key}, ErrNotFound
		}
		return Origin{}, fmt.Errorf("git config --show-origin %s: %w: %s", key, err, strings.TrimSpace(out))
	}

	line := strings.TrimRight(out, "\n")
	origin, value, found := strings.Cut(line, "\t")
	if !found {
		return Origin{Key: key, Value: line}, nil
	}
	return Origin{
		Key:   key,
		Value: value,
		File:  strings.TrimPrefix(origin, "file:"),
	}, nil
}

func (git *Git) InRepo(ctx context.Context) bool {
	out, err := git.runner.Query(ctx, "git", "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func (s Scope) args() []string {
	if s.file != "" {
		return []string{s.flag, s.file}
	}
	if s.flag == "" {
		return nil
	}
	return []string{s.flag}
}

func code(err error) int {
	var exitErr *osexec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
