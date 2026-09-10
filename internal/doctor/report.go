package doctor

import (
	"fmt"
	"io"

	"github.com/semirm-dev/ghu/internal/cli/command"
	"github.com/semirm-dev/ghu/internal/ui"
)

// What a doctor run found, and how it prints.

// GlobalScope is the Profile value of a finding belonging to no one profile.
const GlobalScope = ""

// Check names, stable enough to be matched on in scripts.
const (
	CheckConfig       = "config"
	CheckDir          = "dir"
	CheckKey          = "key"
	CheckProfileFile  = "profile-file"
	CheckInclude      = "includeif"
	CheckIncludeOrder = "includeif-order"
	CheckProbe        = "probe"
	CheckOrphan       = "orphan"
)

const (
	OK Severity = "ok"
	// Warning means something is untidy but git still resolves correctly.
	Warning Severity = "warning"
	// Error means git resolves the wrong identity, or cannot resolve one.
	Error Severity = "error"
)

type Severity string

type Finding struct {
	Profile  string   `json:"profile"`
	Check    string   `json:"check"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Report holds the findings in stable order: profiles in config order, then
// the global checks.
type Report struct {
	Findings []Finding `json:"findings"`
	// ProbeSkipped says why the GitHub probe did not run, empty when it did.
	ProbeSkipped string `json:"-"`
}

func (r Report) Count(s Severity) int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == s {
			n++
		}
	}
	return n
}

func (r Report) HasErrors() bool { return r.Count(Error) > 0 }

// Err is the error a failing report becomes. A non-zero exit is what makes
// doctor usable in CI, and keeping it here stops the command and any other
// caller from wording the same failure two different ways.
func (r Report) Err() error {
	if !r.HasErrors() {
		return nil
	}
	return fmt.Errorf("doctor found %d problem(s)", r.Count(Error))
}

func printReport(e *command.Env, report Report) error {
	if e.JSON {
		// The skip note goes to stderr so stdout stays a bare JSON array.
		printSkip(e.Err, report)
		return e.JSONOut(report.Findings)
	}
	printText(e.Out, report)
	return nil
}

func printText(w io.Writer, report Report) {
	current := "\x00" // no profile name can collide with this
	var rows [][]string

	flush := func() {
		if len(rows) > 0 {
			ui.Table(w, rows)
			rows = nil
		}
	}

	for _, f := range report.Findings {
		if f.Profile != current {
			flush()
			current = f.Profile
			if current != GlobalScope {
				fmt.Fprintf(w, "\n%s\n", ui.Title.Render(current))
			} else {
				fmt.Fprintf(w, "\n%s\n", ui.Title.Render("global"))
			}
		}
		rows = append(rows, []string{"  " + label(f.Severity), ui.Muted.Render(f.Check), f.Message})
	}
	flush()

	fmt.Fprintln(w)
	printSkip(w, report)
	fmt.Fprintln(w, summary(report))
}

func printSkip(w io.Writer, report Report) {
	if report.ProbeSkipped == "" || w == nil {
		return
	}
	fmt.Fprintln(w, ui.Muted.Render(
		"github identity probe skipped ("+report.ProbeSkipped+"); key ownership is unverified"))
}

func label(s Severity) string {
	switch s {
	case Error:
		return ui.Bad.Render("error")
	case Warning:
		return ui.Warn.Render("warn")
	default:
		return ui.Good.Render("ok")
	}
}

func summary(report Report) string {
	errs, warns, oks := report.Count(Error), report.Count(Warning), report.Count(OK)

	line := fmt.Sprintf("%s, %s, %s",
		ui.Bad.Render(count(errs, "error")),
		ui.Warn.Render(count(warns, "warning")),
		ui.Good.Render(count(oks, "ok")))

	if errs == 0 && warns == 0 {
		return ui.Good.Render(ui.Marker) + " " + line
	}
	return line
}

// count renders "1 problem" / "2 problems".
func count(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	if word == "ok" {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
