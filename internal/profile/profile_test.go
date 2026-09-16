package profile_test

// These exercise the real `git config` CLI against an isolated HOME, the same
// way the manual verification in CLAUDE.md does, so a pass here means the
// commands ghu actually shells out to behave the way the domain code expects
// -- not just that the Go code compiles against a mock.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/semirm-dev/ghu/internal/core"
	"github.com/semirm-dev/ghu/internal/core/sys"
	"github.com/semirm-dev/ghu/internal/profile"
)

// TestProfileLifecycle walks a profile through init, add, ls, show, use,
// clear and rm -- the commands profile.go itself says are `ghu`'s whole
// surface -- confirming each leaves ~/.ghu and ~/.gitconfig in the state the
// next command depends on.
func TestProfileLifecycle(t *testing.T) {
	s, layout := newSet(t)
	ctx := context.Background()
	git := sys.NewGit(sys.NewRunner(sys.Opts{}))

	if _, err := s.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(layout.ConfigFile()); err != nil {
		t.Fatalf("Init did not create %s: %v", layout.ConfigFile(), err)
	}

	workDir := filepath.Join(layout.Home, "work")
	p := core.Profile{
		Name:  "work",
		Dir:   workDir,
		User:  "Work Name",
		Login: "work-login",
		Email: "work@example.com",
		Key:   "id_work",
	}

	if _, err := s.Add(ctx, profile.AddOpts{Profile: p}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, ok := s.Config.Find("work"); !ok {
		t.Fatalf("Add did not record %q in the config", "work")
	}

	profileFile := layout.ProfileFile("work")
	if name, err := git.Get(ctx, sys.File(profileFile), "user.name"); err != nil || name != p.User {
		t.Fatalf("generated profile file has user.name = %q, %v; want %q", name, err, p.User)
	}

	includeKey := core.IncludeKey(workDir, layout.Home)
	if got, err := git.Get(ctx, sys.Global(), includeKey); err != nil || got != profileFile {
		t.Fatalf("~/.gitconfig %s = %q, %v; want %q", includeKey, got, err, profileFile)
	}

	initRepo(t, workDir)

	rows := profile.List(s.Config, layout.Home)
	if len(rows) != 1 || !rows[0].Active {
		t.Fatalf("List did not mark %q active from inside its own folder: %+v", p.Name, rows)
	}

	st, err := s.Show(ctx)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if st.Profile != "work" {
		t.Fatalf("Show resolved profile %q, want %q", st.Profile, "work")
	}
	if email, ok := st.Lookup(profile.EmailKey); !ok || email.Value != p.Email {
		t.Fatalf("Show resolved user.email = %+v, want %q", email, p.Email)
	}
	if st.Drift {
		t.Fatalf("Show reported drift for a directory that matches its own profile")
	}

	// `use` and `--clear` are exercised from a repository outside every
	// profile's folder, which is the case they exist for.
	otherDir := filepath.Join(layout.Home, "other")
	initRepo(t, otherDir)

	if _, err := s.Show(ctx); err != nil {
		t.Fatalf("Show outside any profile: %v", err)
	}

	useRes, err := s.Use(ctx, "work")
	if err != nil {
		t.Fatalf("Use: %v", err)
	}
	if useRes.Redundant {
		t.Fatalf("Use reported redundant outside %q's own folder", p.Name)
	}

	st, err = s.Show(ctx)
	if err != nil {
		t.Fatalf("Show after use: %v", err)
	}
	// otherDir is not under any profile's own folder, so Show's directory
	// match still reports no profile; it is the resolved values, via the
	// include.path Use just set, that must now say "work".
	if !st.OverrideManaged {
		t.Fatalf("Show did not recognise its own include.path override: %+v", st)
	}
	if email, ok := st.Lookup(profile.EmailKey); !ok || email.Value != p.Email {
		t.Fatalf("git resolved user.email = %+v after use, want %q", email, p.Email)
	}

	clearRes, err := s.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if !clearRes.Cleared {
		t.Fatalf("Clear did not report clearing an override it just set: %+v", clearRes)
	}

	st, err = s.Show(ctx)
	if err != nil {
		t.Fatalf("Show after clear: %v", err)
	}
	if st.Profile != "" {
		t.Fatalf("Show still resolved profile %q after clear, in a folder no profile covers", st.Profile)
	}

	if _, err := s.Remove(ctx, "work"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := s.Config.Find("work"); ok {
		t.Fatalf("Remove left %q in the config", "work")
	}
	if _, err := os.Stat(profileFile); !os.IsNotExist(err) {
		t.Fatalf("Remove left %s on disk: %v", profileFile, err)
	}
	if _, err := git.Get(ctx, sys.Global(), includeKey); !errors.Is(err, sys.ErrNotFound) {
		t.Fatalf("Remove left %s in ~/.gitconfig: %v", includeKey, err)
	}
}

// A profile file is generated straight from git config --replace-all, so a
// duplicate name reaching Save would silently merge two identities into one
// file. Add refuses before that ever happens.
func TestAddRejectsDuplicateName(t *testing.T) {
	s, _ := newSet(t)
	ctx := context.Background()

	p := core.Profile{Name: "work", Dir: filepath.Join(s.Layout.Home, "work"), User: "W", Email: "w@x.com"}
	if _, err := s.Add(ctx, profile.AddOpts{Profile: p}); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if _, err := s.Add(ctx, profile.AddOpts{Profile: p}); err == nil {
		t.Fatalf("second Add with the same name succeeded, want a rejection")
	}
}

// --dry-run must leave the filesystem exactly as it found it, the rule
// CLAUDE.md calls out as easy to get wrong.
func TestAddDryRunTouchesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	layout := core.LayoutAt(home)
	git := sys.NewGit(sys.NewRunner(sys.Opts{DryRun: true}))

	s, err := profile.Open(layout, git)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	p := core.Profile{Name: "work", Dir: filepath.Join(home, "work"), User: "W", Email: "w@x.com"}
	if _, err := s.Add(context.Background(), profile.AddOpts{Profile: p}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	for _, path := range []string{layout.ConfigFile(), layout.ProfileFile("work"), layout.GitConfig()} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("dry-run Add wrote %s", path)
		}
	}
}

// newSet builds a Set rooted at an isolated home, with a real *sys.Git behind
// it pointed at a throwaway global config, so these tests never touch the
// developer's own ~/.gitconfig.
func newSet(t *testing.T) (*profile.Set, core.Layout) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	layout := core.LayoutAt(home)
	git := sys.NewGit(sys.NewRunner(sys.Opts{}))

	s, err := profile.Open(layout, git)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, layout
}

// initRepo creates a git repository and chdirs the test into it: Show and
// Use both resolve against the working directory, not an argument.
func initRepo(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	t.Chdir(dir)

	if out, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v: %s", dir, err, out)
	}
}
