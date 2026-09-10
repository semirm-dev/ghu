# ghu

Use a different GitHub account in each folder, without switching.

Working under more than one GitHub account means keeping three things in sync per
repository: the commit author name, the commit author email, and the SSH key used
to push. Getting it wrong is silent. Nothing fails, nothing warns, and the commits
land under the wrong account — usually noticed weeks later, on someone else's
contribution graph. `ghu` binds each identity to a directory tree and lets git do
the switching, so a repository gets the right name, email and key because of where
it lives.

## How it works

A profile is one identity — `user`, `email`, ssh `key`, optional commit signing —
bound to one directory tree.

`user` and `login` are different things, and it matters:

| field | is | example |
|---|---|---|
| `user` | the name on your commits — git's `user.name`, a display name | `Semir Mahovkic` |
| `email` | **what GitHub attributes the commit to** | `you@example.com` |
| `login` | your GitHub account, the one in `github.com/<login>` | `semirm-dev` |

Git only ever sees `user` and `email`. `login` is optional and never reaches
git: `ghu doctor` uses it to check that the profile's SSH key really
authenticates as that account.

`ghu` writes two things:

1. One git config file per profile, at `~/.ghu/profiles/<name>.gitconfig`.
2. One `includeIf "gitdir/i:<dir>/"` entry per profile in `~/.gitconfig`, pointing
   at that file.

That is the whole mechanism. Git evaluates `includeIf` itself, per repository, at
the moment it reads its config. No command runs, no daemon watches, nothing hooks
your shell. Clone into `~/code/acme` and the commit identity is already correct.

The ssh key is bound through `core.sshCommand` inside the profile file:

```
ssh -i /Users/you/.ssh/id_acme -o IdentitiesOnly=yes
```

Remote URLs are never rewritten — no `git@github-work:` host aliases — so
repositories cloned before `ghu` existed pick up the right key with no change to
their remotes. `IdentitiesOnly=yes` stops
`ssh-agent` from offering some other key first, which is the usual reason a push
lands as the wrong account.

Nested trees are allowed. `~/code` and `~/code/acme` can both be profiles; the
longest matching prefix wins. `ghu` emits its `includeIf` entries shortest
directory first, because git applies them in file order and the last match wins.

Repositories outside every profile tree opt in explicitly with `ghu profile use`.

## Install

Prebuilt binaries are in [`bin/`](bin) -- pick the one for your machine, make it
executable, and put it on your `PATH`:

| | |
|---|---|
| macOS, Apple Silicon | `bin/ghu-darwin-arm64` |
| macOS, Intel | `bin/ghu-darwin-amd64` |
| Linux, x86-64 | `bin/ghu-linux-amd64` |
| Linux, ARM64 | `bin/ghu-linux-arm64` |
| Windows, x86-64 | `bin/ghu-windows-amd64.exe` |

```bash
curl -LO https://raw.githubusercontent.com/semirm-dev/ghu/main/bin/ghu-darwin-arm64
chmod +x ghu-darwin-arm64
sudo mv ghu-darwin-arm64 /usr/local/bin/ghu
```

`bin/SHA256SUMS` has the checksums; verify with `sha256sum -c SHA256SUMS`.
macOS will quarantine a downloaded binary -- `xattr -d com.apple.quarantine ghu`
clears it.

With Go installed you can skip all that:

```bash
go install github.com/semirm-dev/ghu/cmd/ghu@latest
```

Or build from a checkout:

```bash
make build     # .build/ghu for this machine
make install   # into GOBIN
make release   # bin/, every platform, with checksums
```

Requires git and `ssh-keygen` on `PATH`.

## Quick start

```bash
ghu init
```

Creates `~/.ghu` and offers to collect your first profile. Safe to re-run at any time.

```bash
ghu profile add --name work \
        --dir ~/code/acme \
        --user "Your Name" \
        --login acme-you \
        --email you@acme.example \
        --key ~/.ssh/id_acme
```

Run `ghu profile add` with no flags to fill the same fields in a form.

```bash
ghu ssh generate work
```

Creates the ed25519 keypair at the profile's key path and prints the public half.
Paste that line into <https://github.com/settings/ssh/new>.

```bash
ghu doctor
```

Checks every profile and asks github.com which account each key actually
authenticates as. That last check is the one that catches a mistake before your
commits do.

## Commands

| Command | Flags | What it does |
|---|---|---|
| `ghu` | — | Prints this help. Every action is a command you name. |
| `ghu init` | `--non-interactive`, `--reset` | Creates `~/.ghu` and writes every profile into git's config. Safe to re-run. `--reset` forgets every profile and starts over. |
| `ghu profile add` | `--name`, `--dir`, `--user`, `--login`, `--email`, `--key`, `--sign`, `--generate` | Ties a GitHub account to a folder. With no flags it asks you for each value; it errors instead when there is no terminal. |
| `ghu profile ls` | — | Lists your profiles and the folders they cover, marking the one covering the current directory. Alias: `list`. |
| `ghu profile rm <name>` | — | Removes a profile, so its folder stops using that account. The SSH key is kept. Alias: `remove`. |
| `ghu profile show` | — | Shows which account git will use here, and the file each value came from. |
| `ghu profile use [profile]` | `--clear` | Makes this one repository use a profile, wherever it lives. `--clear` undoes it. |
| `ghu ssh generate <profile>` | `--force` | Creates a profile's SSH key and prints the public half to paste into GitHub. Refuses to replace an existing key without `--force`. |
| `ghu restore` | `--list`, `--from <name>` | Puts `~/.gitconfig` back the way it was before `ghu`. Backs up what it replaces. Profiles are untouched. |
| `ghu doctor` | `--profile <name>`, `--offline` | Checks every profile end to end, including what github.com says the key belongs to. Exits non-zero on any error. |
| `ghu version` | — | Prints the version. |

Run `ghu profile --help` or `ghu ssh --help` for what each accepts.

Global flags, accepted by every command:

| Flag | What it does |
|---|---|
| `--dry-run` | Print every `git config`, `ssh-keygen` and `ssh` invocation that would run, execute none of them, and change nothing on disk. |
| `--json` | Machine-readable output. |
| `--verbose` | Echo each command as it runs. |

`--key` defaults to `~/.ssh/id_<name>` when omitted. `--generate` on `ghu profile add`
runs `ghu ssh generate <name>` as the last step, so the profile and its key are
created in one command and the public key is printed ready to paste.


## What ghu writes

`~/.ghu/config.yaml` is the source of truth. It is the only file you are meant to
edit by hand — run `ghu init` afterwards to reconcile.

```yaml
version: 2
profiles:
    - name: work
      dir: ~/code/acme
      user: Your Name
      login: acme-you
      email: you@acme.example
      key: id_acme          # a bare name means ~/.ssh/id_acme
      sign: true
    - name: personal
      dir: ~/code
      user: Your Name
      login: you
      email: you@personal.example
      key: ~/.ssh/id_personal
      sign: false
```

From that, `~/.ghu/profiles/work.gitconfig`, mode 0600:

```ini
[user]
	name = Your Name
	email = you@acme.example
	signingkey = /Users/you/.ssh/id_acme.pub
[core]
	sshCommand = ssh -i /Users/you/.ssh/id_acme -o IdentitiesOnly=yes
[gpg]
	format = ssh
[commit]
	gpgsign = true
```

The `[gpg]`, `[commit]` and `user.signingkey` entries appear only when
`sign: true`.

And in `~/.gitconfig`:

```ini
[includeIf "gitdir/i:/Users/you/code/"]
	path = /Users/you/.ghu/profiles/personal.gitconfig
[includeIf "gitdir/i:/Users/you/code/acme/"]
	path = /Users/you/.ghu/profiles/work.gitconfig
```

Three details in there are deliberate.

**Keys live in `~/.ssh`.** That is where OpenSSH looks on Linux, macOS and
Windows alike — Windows OpenSSH uses `%USERPROFILE%\.ssh` — so `ghu` takes a
bare `key: private` to mean `~/.ssh/private`, and refuses a key anywhere else.
A key outside it is either an absolute path that will not survive being moved
between machines, or a relative one that resolves against whatever directory
you ran from.

**Every path is absolute.** Git expands `~` in `include.path` and in `includeIf`
patterns, but `core.sshCommand` only reaches a shell when it contains shell
metacharacters, so a `~` there is not reliably expanded. One rule — always
absolute — beats remembering which fields are safe. The config file keeps `~` for
readability; expansion happens when the generated files are written.

**`gitdir/i`, not `gitdir`.** macOS and Windows default to case-insensitive
filesystems, where the case-sensitive form silently fails to match.

**The trailing slash** is what makes a pattern match the tree recursively instead
of one directory.

`ghu` never writes git config syntax by hand. Every value goes in through
`git config --file …` or `git config --global --replace-all …`, so git owns the
quoting, the section merging and the idempotency. Run any command with `--dry-run`
to see the exact list.

## Safety

- **`~/.ssh/config` is never read or written.** Not on install, not on
  reconcile, not on removal.
- **SSH keys are never deleted except by `ghu ssh generate --force`.** `ghu profile rm`
  drops the profile, its generated file and its `includeIf` entry, and tells you
  where the key was left. A key may be trusted by hosts `ghu` knows nothing
  about, and deleting one cannot be undone. `--force` is the single exception,
  because replacing a key means removing the old one first; it says so before
  and after, and under `--dry-run` it says so without touching anything.
- **`~/.gitconfig` is backed up before the first write of each run.** It is the
  only file `ghu` modifies that you did not ask it to create. Backups come in
  two kinds:

  - `~/.ghu/backups/gitconfig.original` — the **pivot**: your `~/.gitconfig`
    exactly as it stood before `ghu` ever touched it. Written once, never
    rewritten, never pruned. This is the copy worth having, so it is kept
    forever.
  - `~/.ghu/backups/gitconfig.<timestamp>` — rolling snapshots from later runs.
    The ten most recent are kept; older ones are pruned.

  To start over without unwinding `~/.gitconfig`:

  ```bash
  ghu init --reset
  ```

  That forgets every profile and removes the files and `includeIf` entries they
  produced. Your backups and ssh keys are kept, anything else in `~/.gitconfig`
  is left alone, and the config that held the profiles is saved alongside itself
  first — so it is recoverable.

  To go back to how things were before `ghu`:

  ```bash
  ghu restore
  ```

  That puts the pivot back, after taking one more snapshot of the file it is
  replacing, so restoring is not a one-way door either. `ghu restore --list`
  shows every copy it holds, and `--from <name>` picks one. Your profiles are
  untouched — `ghu init` applies them again.

  If you had no `~/.gitconfig` at all when `ghu` first ran, the pivot is an
  empty file — restoring it gives you an empty global config, which git treats
  the same as none.

Reconciling also only ever removes `includeIf` entries that point into
`~/.ghu/profiles`. An entry you wrote yourself is left exactly where it is.

`--dry-run` is part of that story rather than a convenience:

```bash
ghu profile add --name side --dir ~/code/side --user side-you --email you@side.example --dry-run
```

prints every command in order and runs none of them.

## Repositories outside a profile tree

A repository that does not live under any profile's directory opts in explicitly:

```bash
cd ~/one-off-clone
ghu profile use work
```

That writes a single local key:

```bash
git config --local include.path ~/.ghu/profiles/work.gitconfig
```

One key, pulling in the whole profile file, fully reversible:

```bash
ghu profile use --clear
```

`--clear` refuses if `include.path` holds a value `ghu` did not write, and tells
you the `git config --local --unset` command to run instead.

## Troubleshooting

**Start with `ghu profile show`.** It reports what git resolves for the current
directory — `user.name`, `user.email`, `core.sshCommand`, `user.signingkey`,
`commit.gpgsign` — and names the file each value came from, straight out of
`git config --show-origin`. It reads git, never `ghu`'s own config, so it can
disagree with the profile you expected. That disagreement is the answer. It also
flags drift: a directory covered by a profile where git nonetheless resolves a
different `user.email`.

**Then `ghu doctor`.** It checks each profile's config entry, its directory, its
key permissions, its generated file, its `includeIf` entry and that entry's
position in the ordering — then opens an ssh connection with that specific key and
reports which GitHub login answers. That probe is the only check that proves a key
belongs to the account the profile claims. `--offline` skips it; `--profile <name>`
narrows the run; a non-zero exit makes it usable in CI.

**Symlinks.** Git resolves symlinks before matching `gitdir`, so a repository
reached through a symlink into a profile tree matches on its *real* path, not the
path you typed. If `~/work` is a symlink to `/Volumes/data/work`, point the profile
at `/Volumes/data/work`. `ghu profile show` resolves symlinks the same way git does, so it
will show you the truth rather than what you hoped.

**Commits show as unverified with `sign: true`.** GitHub keeps authentication keys
and signing keys in two separate lists. The same key has to be added twice: once at
<https://github.com/settings/ssh/new> as an Authentication Key, and again at the
same page with the type set to Signing Key. A key added only for authentication
will push fine and leave every commit unverified, with no error anywhere to explain
it. `ghu ssh generate` prints this reminder for signing profiles.

**Nothing seems to apply.** Check that the repository is actually under the
profile's `dir` (`ghu profile show` prints the resolved directory), and remember that
`includeIf` matches the repository's location, not your shell's — a repo outside
every tree needs `ghu profile use`.

## Development

```bash
make build       # build .build/ghu
make install     # install into GOBIN
make lint        # gofmt, imports, declaration order, go vet
make order       # rewrite declarations into the house order
make test        # go test ./... -race -count=1
make release     # cross-compile bin/ for every platform
make clean       # remove build output
```

The test suite was removed while the package layout was being reworked; what
is left covers key-path handling, which is the part that differs between Linux,
macOS and Windows and fails silently when it is wrong.

`ghu` is organised by feature. Each package under `internal/` owns the commands
that drive it -- `ghu profile add` is declared in `internal/profile/`, not in
`internal/cli/` -- so everything about one command is in one place. No feature
package imports another; where a command needs another feature's work,
`internal/cli` passes it in as a callback.

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
  command/        the context a command runs in
  cli/            assembles the command tree; owns only `ghu version`
  tui/  ui/       the profile form, and the shared styles
```

The core returns values and never renders: operations hand back structs that
are already JSON-tagged, because those structs are the `--json` output.
Presentation lives in the commands, which is what keeps every operation
runnable without a terminal.

`CLAUDE.md` has the conventions in full -- file layout, declaration order, and
the details of git's `includeIf` handling that are easy to get wrong.
