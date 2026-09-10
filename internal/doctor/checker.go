package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/semirm-dev/ghu/internal/kernel"
	"github.com/semirm-dev/ghu/internal/sys"
)

// The online half of doctor: asking GitHub what identity a profile's key
// actually authenticates as, which is the one thing no local check can tell.

// MaxConcurrentProbes caps how many ssh sessions doctor opens to github.com at
// once. The profile count is user configuration, so an unbounded fan-out would
// let a large ~/.ghu/config.yaml open a connection per profile and look like a
// burst of failed auth attempts from one address.
const MaxConcurrentProbes = 4

// The checker: what one doctor run examines, and every check it makes -- the
// ones it can decide from ~/.ghu's config, the files on disk and ~/.gitconfig
// as git itself parses it, and the one only github.com can answer.

// checker is one doctor run: the collaborators it reads the world with, the
// config it is checking, and the ghu-owned includeIf entries, read once up
// front because the ordering check needs to see all of them at once.
//
// It is one type rather than a Checker holding collaborators and a second
// bundle holding the run: nothing outside this package builds one, and a run
// is the only thing a checker ever does.
type checker struct {
	layout kernel.Layout
	git    *sys.Git
	ssh    *sys.SSH

	cfg   kernel.Config
	owned []entry
}

type entry struct {
	key   string
	value string
	// profileName is taken from the file the entry points at, so an entry
	// naming a profile that no longer exists can still be identified.
	profileName string
	// index is the position among ghu-owned entries only, in file order.
	index int
}

type probe struct {
	ran   bool
	login string
	err   error
}

// check returns an error only when the run itself could not proceed -- an unknown
// profile name, or a ~/.gitconfig git refuses to read. Failed checks are
// findings, not errors; callers decide what to do with Report.HasErrors.
func (c checker) check(ctx context.Context, opts Opts) (Report, error) {
	targets, err := targets(c.cfg, opts.Profile)
	if err != nil {
		return Report{}, err
	}

	// git lists includeIf entries in file order, which is the order git itself
	// applies them. That order is the subject of the ordering check, so it is
	// read once and shared by every per-profile check below.
	entries, err := c.git.GetRegexp(ctx, sys.Global(), `^includeIf\.`)
	if err != nil {
		return Report{}, fmt.Errorf("reading includeIf entries from %s: %w", c.layout.GitConfig(), err)
	}

	// The receiver is a value, so filling in what this run found leaves the
	// checker the caller built untouched.
	c.owned = ownedEntries(c.layout, entries)

	report := Report{Findings: []Finding{}}
	if len(c.cfg.Profiles) == 0 {
		report.Findings = append(report.Findings, Finding{
			Profile:  GlobalScope,
			Check:    CheckConfig,
			Severity: Warning,
			Message:  "no profiles configured; run `ghu init` to add one",
		})
	}

	report.ProbeSkipped = c.probeSkipReason(opts)
	logins := c.probeAll(ctx, targets, report.ProbeSkipped != "")

	for i, p := range targets {
		findings := c.checkProfile(p)
		if logins[i].ran {
			findings = append(findings, c.probeFinding(p, logins[i]))
		}
		report.Findings = append(report.Findings, findings...)
	}

	report.Findings = append(report.Findings, c.checkOrphans()...)
	return report, nil
}

func (e entry) matchesProfile(name string) bool {
	return strings.EqualFold(e.profileName, name)
}

// checkProfile runs every check that needs no network.
func (c checker) checkProfile(p kernel.Profile) []Finding {
	var out []Finding
	add := func(check string, sev Severity, format string, args ...any) {
		out = append(out, Finding{
			Profile:  p.Name,
			Check:    check,
			Severity: sev,
			Message:  fmt.Sprintf(format, args...),
		})
	}
	home := c.layout.Home

	if err := p.Validate(); err != nil {
		add(CheckConfig, Error, "%s", err)
	} else {
		add(CheckConfig, OK, "config entry is valid")
	}

	// A profile pointing at a directory that was moved or deleted matches
	// nothing, silently -- git simply never applies the include.
	dir := kernel.Expand(p.Dir, home)
	switch info, err := os.Stat(dir); {
	case os.IsNotExist(err):
		add(CheckDir, Error, "directory %s does not exist, so nothing will ever match this profile", dir)
	case err != nil:
		add(CheckDir, Error, "cannot stat %s: %s", dir, err)
	case !info.IsDir():
		add(CheckDir, Error, "%s is not a directory", dir)
	default:
		add(CheckDir, OK, "directory %s exists", dir)
	}

	keyPath := kernel.Expand(p.Key, home)
	if strings.HasSuffix(keyPath, sys.PublicKeySuffix) {
		// Checked as a private key this yields two findings sharing one cause:
		// 0644 permissions, which are right for a public key, and a missing
		// .pub.pub. Report the cause.
		add(CheckKey, Error, "%s is a public key; point this profile at the private half, %s",
			keyPath, strings.TrimSuffix(keyPath, sys.PublicKeySuffix))
	} else {
		problems := sys.CheckKey(keyPath)
		for _, problem := range problems {
			add(CheckKey, keySeverity(problem, p), "%s: %s", problem.Path, problem.Message)
		}
		if len(problems) == 0 {
			add(CheckKey, OK, "key %s exists with mode %04o", keyPath, sys.KeyMode)
		}
	}

	generated := c.layout.ProfileFile(p.Name)
	if _, err := os.Stat(generated); err != nil {
		add(CheckProfileFile, Error, "%s is missing; run `ghu init` to regenerate it", generated)
	} else {
		add(CheckProfileFile, OK, "%s exists", generated)
	}

	out = append(out, c.checkInclude(p)...)
	return out
}

func (c checker) checkInclude(p kernel.Profile) []Finding {
	var out []Finding
	add := func(check string, sev Severity, format string, args ...any) {
		out = append(out, Finding{
			Profile:  p.Name,
			Check:    check,
			Severity: sev,
			Message:  fmt.Sprintf(format, args...),
		})
	}

	wantKey := kernel.IncludeKey(p.Dir, c.layout.Home)
	wantValue := c.layout.ProfileFile(p.Name)

	var mine *entry
	var byKey *entry
	for i := range c.owned {
		if c.owned[i].matchesProfile(p.Name) {
			mine = &c.owned[i]
		}
		// git lowercases the section name on output (includeif.), and gitdir/i
		// is a case-insensitive pattern, so the whole key is compared without
		// regard to case.
		if strings.EqualFold(c.owned[i].key, wantKey) {
			byKey = &c.owned[i]
		}
	}

	switch {
	case mine == nil && byKey == nil:
		add(CheckInclude, Error,
			"no includeIf entry in %s for this profile; run `ghu init` to write one",
			c.layout.GitConfig())
		return out
	case mine == nil:
		// Some other profile's file is installed under this profile's
		// directory: repositories in the tree get the wrong identity.
		add(CheckInclude, Error,
			"includeIf entry for %s points at %s, expected %s",
			kernel.Expand(p.Dir, c.layout.Home), byKey.value, wantValue)
		return out
	case !strings.EqualFold(mine.key, wantKey):
		// The profile's dir was changed in the config but ~/.gitconfig was
		// never reconciled, so the tree the entry covers is stale.
		add(CheckInclude, Error,
			"includeIf entry is %s, expected %s; run `ghu init` to reconcile",
			mine.key, wantKey)
	case mine.value != wantValue:
		add(CheckInclude, Error,
			"includeIf entry for %s points at %s, expected %s",
			kernel.Expand(p.Dir, c.layout.Home), mine.value, wantValue)
	default:
		add(CheckInclude, OK, "includeIf entry matches %s", wantValue)
	}

	out = append(out, c.orderFinding(p, *mine))
	return out
}

// git applies includeIf entries in file order and the last match wins, so the
// entries must run shortest directory first. Get it backwards and a nested
// profile is silently overridden by the parent tree that contains it: git
// resolves without complaint, and commits land under the wrong account.
func (c checker) orderFinding(p kernel.Profile, mine entry) Finding {
	finding := Finding{Profile: p.Name, Check: CheckIncludeOrder}

	want := c.expectedOrder()
	wantIndex := -1
	for i, name := range want {
		if strings.EqualFold(name, p.Name) {
			wantIndex = i
			break
		}
	}

	if wantIndex == mine.index {
		finding.Severity = OK
		finding.Message = fmt.Sprintf("includeIf entry is in order (%d of %d)", mine.index+1, len(want))
		return finding
	}

	// Being out of position only costs something when an ancestor tree is
	// applied afterwards, so name that profile rather than the position.
	if shadower, ok := c.shadowedBy(p, mine); ok {
		finding.Severity = Error
		finding.Message = fmt.Sprintf(
			"includeIf entry is applied before %q, whose tree %s contains %s; git's last match wins, so this profile silently loses to %q",
			shadower.Name, kernel.Expand(shadower.Dir, c.layout.Home), kernel.Expand(p.Dir, c.layout.Home), shadower.Name)
		return finding
	}

	finding.Severity = Warning
	finding.Message = fmt.Sprintf(
		"includeIf entry is at position %d of %d, expected %d; entries must be shortest-directory-first, run `ghu init` to reconcile",
		mine.index+1, len(c.owned), wantIndex+1)
	return finding
}

// expectedOrder is restricted to the entries actually present in ~/.gitconfig,
// so that a missing entry does not shift everything after it.
func (c checker) expectedOrder() []string {
	present := make(map[string]bool, len(c.owned))
	for _, e := range c.owned {
		present[strings.ToLower(e.profileName)] = true
	}

	var out []string
	for _, inc := range kernel.Includes(c.cfg, c.layout.Home, c.layout.ProfileFile) {
		if present[strings.ToLower(inc.Profile)] {
			out = append(out, inc.Profile)
		}
	}
	return out
}

// shadowedBy returns the profile that overrides p, if any: one whose tree
// contains p's directory and whose includeIf entry is applied later.
func (c checker) shadowedBy(p kernel.Profile, mine entry) (kernel.Profile, bool) {
	for _, e := range c.owned {
		if e.index <= mine.index || e.matchesProfile(p.Name) {
			continue
		}
		other, ok := c.cfg.Find(e.profileName)
		if !ok {
			continue
		}
		// Match against a single candidate answers "is p.Dir inside other.Dir",
		// using the same containment rule git's gitdir/i patterns follow.
		if _, contains := kernel.Match([]kernel.Profile{other}, p.Dir, c.layout.Home); contains {
			return other, true
		}
	}
	return kernel.Profile{}, false
}

// Orphaned entries -- ghu-owned includeIf entries whose profile is gone from
// the config -- are harmless until the deleted profile's directory is reused,
// at which point git applies an identity ghu no longer knows about.
func (c checker) checkOrphans() []Finding {
	var out []Finding
	for _, e := range c.owned {
		if _, ok := c.cfg.Find(e.profileName); ok {
			continue
		}
		out = append(out, Finding{
			Profile:  GlobalScope,
			Check:    CheckOrphan,
			Severity: Warning,
			Message: fmt.Sprintf(
				"%s in %s points at %s, but no profile %q is configured; run `ghu init` to reconcile",
				e.key, c.layout.GitConfig(), e.value, e.profileName),
		})
	}
	return out
}

// probeSkipReason is empty when the probe will run.
func (c checker) probeSkipReason(opts Opts) string {
	switch {
	case opts.Offline:
		return "--offline"
	case c.ssh.DryRun():
		return "--dry-run"
	default:
		return ""
	}
}

// probeAll runs the GitHub probes concurrently, because they are
// network-bound and independent. Results stay indexed by profile so the report
// keeps config order regardless of which probe answers first, and each
// goroutine owns one index, so no lock is needed to collect them.
func (c checker) probeAll(ctx context.Context, targets []kernel.Profile, skip bool) []probe {
	results := make([]probe, len(targets))
	if skip {
		return results
	}

	// A counting semaphore: acquiring is a send, releasing a receive.
	sem := make(chan struct{}, MaxConcurrentProbes)

	var wg sync.WaitGroup
	for i, p := range targets {
		keyPath := kernel.Expand(p.Key, c.layout.Home)
		// Probing with a key that is not there only produces a second, less
		// useful report of the same missing file.
		if _, err := os.Stat(keyPath); err != nil {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				// Cancelled while queued: report it rather than opening a
				// connection nobody is waiting for any more.
				results[i] = probe{ran: true, err: ctx.Err()}
				return
			}

			login, err := c.ssh.Probe(ctx, keyPath)
			results[i] = probe{ran: true, login: login, err: err}
		}()
	}
	wg.Wait()

	return results
}

// The probe is the only check that proves a key belongs to the account the
// profile claims, so a mismatch is the worst thing doctor can find: git is
// configured, resolving, and pushing as somebody else.
func (c checker) probeFinding(p kernel.Profile, result probe) Finding {
	finding := Finding{Profile: p.Name, Check: CheckProbe}

	switch {
	case result.err != nil:
		finding.Severity = Error
		finding.Message = fmt.Sprintf("github.com rejected %s: %s", kernel.Expand(p.Key, c.layout.Home), result.err)
	case !p.ClaimsLogin():
		// Without a login there is nothing to compare against, so report what
		// answered rather than inventing a claim from user.name -- that is a
		// display name, and holding it to a GitHub account name fails for
		// everyone whose real name is not their username.
		finding.Severity = OK
		finding.Message = fmt.Sprintf(
			"github.com answered as %q; set `login: %s` on this profile to have doctor check it",
			result.login, result.login)
	case !strings.EqualFold(result.login, p.Login):
		finding.Severity = Error
		finding.Message = fmt.Sprintf(
			"github.com answered as %q but this profile claims %q; pushes from %s would be attributed to %q",
			result.login, p.Login, kernel.Expand(p.Dir, c.layout.Home), result.login)
	default:
		finding.Severity = OK
		finding.Message = fmt.Sprintf("github.com answered as %q", result.login)
	}
	return finding
}

// ownedEntries keeps only the includeIf entries pointing into ~/.ghu/profiles,
// so that entries the user wrote by hand are never reported on.
func ownedEntries(l kernel.Layout, all []sys.KeyValue) []entry {
	var out []entry
	for _, kv := range all {
		if !l.OwnsProfilePath(kv.Value) {
			continue
		}
		out = append(out, entry{
			key:         kv.Key,
			value:       kv.Value,
			profileName: strings.TrimSuffix(filepath.Base(kv.Value), kernel.ProfileFileSuffix),
			index:       len(out),
		})
	}
	return out
}

// A missing or world-readable private key stops ssh outright; a missing public
// key only matters when the profile signs commits with it.
func keySeverity(problem sys.KeyProblem, p kernel.Profile) Severity {
	if strings.HasSuffix(problem.Path, sys.PublicKeySuffix) && !p.Sign {
		return Warning
	}
	return Error
}
