# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What ghu is

A CLI that binds a GitHub account to a directory tree, so repositories under
`~/work` commit and push as your work account and repositories under
`~/personal` as your personal one, with nothing to run or remember.

It does this by writing one git config file per profile under
`~/.ghu/profiles/`, and one `includeIf` entry per profile in `~/.gitconfig`.
Repositories outside every profile's folder opt in with `ghu profile use`.

## Commands

```bash
make build      # .build/ghu, with the version stamped from VERSION
make install    # into GOBIN
make lint       # gofmt, imports, declaration order, vet -- run before committing
make order      # rewrite declarations into the house order (see below)
make test       # go test ./... -race -count=1
make tidy       # go mod tidy
```

`goimports` is a module tool (`go tool goimports`), so `make lint` needs no
separate install. To fix grouping rather than just report it:

```bash
go tool goimports -local github.com/semirm-dev/ghu -w .
```

**There is almost no test suite.** It was removed deliberately while the
package layout was in flux; `internal/kernel/keypath_test.go` is what came
back, because key-path handling is the part that differs across platforms and
fails silently. Write new ones black-box (`package profile_test`) as the rest
of the codebase was. A single one runs with
`go test ./internal/profile/ -run TestName -count=1`.

To stub subprocess execution, give `sys.Runner` an unexported `exec` func field
defaulting to the real one — not an interface with a single implementation.

Manual verification is the current substitute, and it needs an isolated home or
it will rewrite your own `~/.gitconfig`:

```bash
H=$(mktemp -d)
HOME=$H GIT_CONFIG_GLOBAL=$H/.gitconfig GIT_CONFIG_SYSTEM=/dev/null .build/ghu init --non-interactive
```

`--dry-run` prints the git and ssh commands without running them, and touches
nothing on disk.

## Architecture

```
cmd/ghu/          main
internal/
  profile/        ghu init, and ghu profile add|ls|show|use|rm
  ssh/            ghu ssh generate
  doctor/         ghu doctor
  backup/         ghu restore
  kernel/         the model, ~/.ghu/config.yaml, the layout, directory
                  resolution, the git-config contract, the reconciler
    sys/          subprocesses, git config, ssh, atomic writes
    command/      the context a command runs in
  cli/            assembles the command tree; owns only `ghu version`
  tui/  ui/       the profile form, and the shared lipgloss styles
```

Everything but `cmd/ghu` is under `internal/`, so nothing outside the module
can import it. ghu ships a binary, not a library.

### The rules that matter

**A feature owns its commands.** `ghu profile add` is declared in
`internal/profile/`, not in `cli/`. Each feature package exports its cobra
commands (`profile.Command`, `ssh.Command`, `doctor.Commands`) and `cli` only
assembles them. A feature's whole external surface should be its commands and
little else.

**No feature imports another feature.** `profile`, `ssh` and `doctor` depend on
`command`, `kernel`, `sys` and `ui` — never on each other. Where one command
needs another (`ghu profile add --generate` must print what `ghu ssh generate`
prints), `cli` passes the action in as a `command.Action` callback. Preserve
this: it is why `kernel/command` exists at all — `Env` cannot live in `cli`, because
features need it and `cli` imports every feature.

**`kernel` holds contracts, not behaviour looking for a home.** `Profile` and
`Config` are the aggregate every feature reads; `Layout` is where ghu keeps
files; `Expand`/`Match`/`Ordered` are directory resolution;
`Entries`/`Includes`/`IncludeKey` are the contract between the reconciler that
*writes* a profile's git config and doctor that *verifies* it.
`kernel/config.go` is the only file in the tree that imports a yaml package.

**The core returns values and never renders.** Operations hand back structs
that are already JSON-tagged, because those structs are the `--json` output.
A caller wanting a profile passes a finished one — collecting it from flags or
a form is the command's job, which is what keeps `ghu profile add` runnable
without a terminal.

**One `sys.Runner` (in `kernel/sys`), taken directly — there is no interface over it.** `Git` and
`SSH` hold a `*Runner`. If tests need to stub subprocess execution, give
`Runner` an unexported `exec` func field defaulting to the real one, rather
than reintroducing a one-implementation interface.

**The Runner is the only place a dry run is recorded.** `Git` and `SSH` answer
`DryRun()` by asking it. Never store a copy: it once lived in
three structs, and the pair that costs you something is `--force` with
`--dry-run`, where key generation deletes the old key directly and lets the
runner swallow the `ssh-keygen` that would have replaced it.

**ghu never assembles git config syntax.** Every write goes through the
`git config` CLI in `kernel/sys/git.go`, so git owns quoting, section merging and
idempotency.

## Declaration order

Every file orders its top-level declarations this way. `make lint` fails if one
does not, so it is checked rather than remembered:

1. constants
2. variables
3. exported types
4. unexported types
5. exported functions
6. exported methods
7. unexported methods
8. unexported functions

Within a group, source order is kept, so related declarations stay adjacent.
After adding or moving anything, run:

```bash
make order      # rewrites every file to the order above, then gofmt
```

`scripts/order.py` does the work and takes `--check` to report without
rewriting, which is what `make lint` calls.

## File and naming conventions

**One file per command**, holding the operation, the cobra command and the
output it prints — `ghu profile add` is entirely in `profile/add.go`, next to
`Set.Add`.

**Anything two files in a package share moves to that package's main file**
(`profile.go`, `doctor.go`, `kernel.go`, `cli.go`, `kernel/sys/runner.go`). A command
file holds only what is specific to that command; nothing reaches sideways into
another command's file. A main file *using* its parts is fine and expected.

**A type's methods live in the file with the type.**

**Imports are grouped** stdlib / third-party / local, enforced by `make lint`.

**No stuttering** (guidelines §7): the repository-scoped type is not
`repo.Repo`, the options type is `doctor.Opts` not `doctor.DoctorOpts`.

**Error strings are lowercase and unpunctuated**, and `errors.New` is used
wherever there are no format verbs.

The house style is [these Go guidelines](https://gist.github.com/semirm-dev/d2317ef1ef3c822935b42a2b5662cea4).

## Domain details that are easy to get wrong

**Keys live in `~/.ssh`, on every platform.** `kernel.KeyDir` is the one
statement of that. A bare `key: private` means `~/.ssh/private`; anything
outside is refused by `Profile.Validate`. Read a key through `Profile.KeyPath()`,
never the raw field, or a bare name reaches git unresolved. Both separators are
treated as separators regardless of host, because config.yaml is portable and a
key written on Windows must mean the same on Linux.

**`user` and `login` are different fields and must stay that way.** `user` is
git's `user.name`, a display name on commits. `login` is the GitHub account,
which git never sees and only `doctor`'s probe reads. They were one field once,
which made `ghu doctor` report an error for anyone whose real name was not
their GitHub username. A profile with no `login` makes no claim, so the probe
reports which account answered instead of judging it.

**`includeIf` order is load-bearing.** git applies entries in file order and the
*last* match wins, so ghu writes them shortest-directory-first — that is what
makes a nested profile beat the parent tree containing it. `kernel.Ordered` and
`doctor`'s `includeif-order` check both exist for this.

**Paths in `kernel/paths.go` are slash-separated, not filepath-separated,**
because they end up inside git config values and git wants forward slashes in
`includeIf` patterns on every platform, Windows included.

**`gitdir/i` is used unconditionally** — macOS and Windows default to
case-insensitive filesystems, where plain `gitdir` silently fails to match.

**`core.sshCommand` gets an absolute path.** git expands `~` in `include.path`
and in `includeIf` patterns, but not reliably there.

**ghu never edits `~/.ssh/config`, and never deletes an SSH key** except under
`ghu ssh generate --force`. Removing a profile is a configuration change, not a
revocation.

**`ghu init --reset` starts over without a destructive path of its own.** It
empties the config and reconciles: `Apply` already removes the generated files
and includeIf entries of profiles that are no longer configured, so reset is
the same code every other change takes. It backs the config up first, and never
touches backups or ssh keys.

**`ghu restore` is the way back**, and restoring is itself a write, so it
snapshots what it replaces first.

**`~/.gitconfig` is the only file ghu touches that the user did not ask it to
create**, so it is backed up before every write. The first backup becomes the
pivot (`~/.ghu/backups/gitconfig.original`), kept forever and never pruned;
later runs add rolling snapshots.

**`--dry-run` must leave the filesystem exactly as it found it**, including
backups and directory creation — not just the commands the runner suppresses.

## Committing

`make lint` and `go build ./...` should both be clean. Commit messages in this
repository explain *why* a change was made, not just what changed; match that.
