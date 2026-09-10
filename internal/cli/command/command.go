// Package command is the context every ghu command runs in: the flags, the
// writers, and the collaborators the features are driven with.
//
// It exists so that each feature can own its own commands. A command lives
// beside the code it drives -- `ghu profile add` in profile, `ghu ssh generate`
// in ssh -- and they all need the same handful of things, so that handful lives
// here rather than in the package that assembles the tree.
//
// It sits under cli because it belongs to that layer, not to the domain: it
// holds writers and flags, and knows what a cobra command needs. Features
// import it without importing cli, which is what keeps the one direction that
// works: cli imports every feature, and no feature imports cli.
package command

import (
	"context"
	"encoding/json"
	"io"

	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/kernel/sys"
)

// Action is a command one feature exposes to another -- `ghu profile add
// --generate` running ssh's generate -- so that the output is worded
// identically wherever it is asked for, without the two importing each other.
type Action func(ctx context.Context, name string) error

// Env is one invocation's wiring. It is a composition root, not a domain
// object: the feature packages each take only what they need, and the commands
// assemble them from here.
type Env struct {
	Layout kernel.Layout
	Config kernel.Config
	Git    *sys.Git
	SSH    *sys.SSH

	Out, Err io.Writer

	JSON    bool
	DryRun  bool
	Verbose bool
}

// JSONOut writes v as the indented JSON every --json command emits.
func (e *Env) JSONOut(v any) error {
	enc := json.NewEncoder(e.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
