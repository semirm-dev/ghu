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

// Call is one subprocess invocation.
type Call struct {
	Name string
	Args []string
}

// Opts configures a Runner. The zero value runs commands silently.
type Opts struct {
	// Out receives the --verbose echo and the --dry-run listing. A nil Out
	// prints nothing.
	Out io.Writer

	// Verbose echoes every command as it runs.
	Verbose bool

	// DryRun prints mutating commands instead of executing them.
	DryRun bool
}

// Runner executes subprocesses.
type Runner struct {
	out     io.Writer
	verbose bool
	dryRun  bool
}

func NewRunner(opts Opts) *Runner {
	return &Runner{out: opts.Out, verbose: opts.Verbose, dryRun: opts.DryRun}
}

func (c Call) String() string {
	parts := make([]string, 0, len(c.Args)+1)
	parts = append(parts, c.Name)
	for _, a := range c.Args {
		if strings.ContainsAny(a, " \t\"'") {
			a = fmt.Sprintf("%q", a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// DryRun reports whether mutating commands are suppressed. Callers that also
// change the filesystem directly -- which no Runner can intercept -- ask here
// rather than tracking a flag of their own.
func (r *Runner) DryRun() bool { return r.dryRun }

// Run executes a command that changes something. Under DryRun it prints the
// command and reports success without running it, so callers need no
// conditional of their own.
func (r *Runner) Run(ctx context.Context, name string, args ...string) (string, error) {
	call := Call{Name: name, Args: args}

	if r.dryRun {
		r.print("would run: %s\n", call)
		return "", nil
	}
	if r.verbose {
		r.print("+ %s\n", call)
	}
	return run(ctx, name, args...)
}

// Query executes a command that only reads. It runs even under DryRun: a dry
// run that could not read the current state would have nothing to report.
func (r *Runner) Query(ctx context.Context, name string, args ...string) (string, error) {
	if r.verbose {
		r.print("+ %s\n", Call{Name: name, Args: args})
	}
	return run(ctx, name, args...)
}

func (r *Runner) print(format string, call Call) {
	if r.out == nil {
		return
	}
	fmt.Fprintf(r.out, format, call)
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := osexec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
