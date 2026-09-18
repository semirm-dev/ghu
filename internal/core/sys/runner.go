// Package sys is everything in ghu that touches the world outside the process:
// running commands, the git config CLI, ssh key generation, and the two file
// operations that need care. It knows nothing about profiles, which is what
// keeps the domain package free of I/O.
//
// There is one Runner and no interface over it. --dry-run is a property of a
// run, not a kind of runner: the same command list is built either way, and
// DryRun only decides whether the mutating ones reach the system. Git and SSH
// take a *Runner directly -- an interface with one implementation is a name
// for a type that already has one.
//
// The Runner is the single source of truth for whether a run may change
// anything. Git and SSH answer DryRun() by asking it, and callers that touch
// the filesystem directly -- which no Runner can intercept -- ask them. Nothing
// keeps a copy of the flag: it lived in three structs at once, and the pair
// that costs you something is --force with --dry-run, where key generation
// removes the old key itself and lets the runner swallow the ssh-keygen call
// that would have replaced it.
package sys

import (
	"context"
	"fmt"
	"io"
	osexec "os/exec"
	"strings"
)

// Runner executes subprocesses.
type Runner struct {
	out     io.Writer
	verbose bool
	dryRun  bool
}

// call is one subprocess invocation: a name and args to run, and, via
// String, the quoted form the --verbose/--dry-run echo line prints.
type call struct {
	name string
	args []string
}

// NewRunner builds a Runner. out receives the --verbose echo and the
// --dry-run listing; a nil out prints nothing.
func NewRunner(out io.Writer, verbose, dryRun bool) *Runner {
	return &Runner{out: out, verbose: verbose, dryRun: dryRun}
}

// DryRun reports whether mutating commands are suppressed. Callers that also
// change the filesystem directly -- which no Runner can intercept -- ask here
// rather than tracking a flag of their own.
func (r *Runner) DryRun() bool { return r.dryRun }

// Run executes a command that changes something. Under DryRun it prints the
// command and reports success without running it, so callers need no
// conditional of their own.
func (r *Runner) Run(ctx context.Context, name string, args ...string) (string, error) {
	c := call{name: name, args: args}

	if r.dryRun {
		r.print("would run: %s\n", c)
		return "", nil
	}
	if r.verbose {
		r.print("+ %s\n", c)
	}
	return run(ctx, c)
}

// Query executes a command that only reads. It runs even under DryRun: a dry
// run that could not read the current state would have nothing to report.
func (r *Runner) Query(ctx context.Context, name string, args ...string) (string, error) {
	c := call{name: name, args: args}
	if r.verbose {
		r.print("+ %s\n", c)
	}
	return run(ctx, c)
}

func (c call) String() string {
	parts := make([]string, 0, len(c.args)+1)
	parts = append(parts, c.name)
	for _, a := range c.args {
		if strings.ContainsAny(a, " \t\"'") {
			a = fmt.Sprintf("%q", a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

func (r *Runner) print(format string, c call) {
	if r.out == nil {
		return
	}
	fmt.Fprintf(r.out, format, c)
}

func run(ctx context.Context, c call) (string, error) {
	cmd := osexec.CommandContext(ctx, c.name, c.args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
