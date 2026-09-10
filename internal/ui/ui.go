// Package ui holds the styles shared by the TUI and by plain command output,
// so that both halves of ghu look like one program.
package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
)

// Marker is the glyph placed beside the active profile.
const Marker = "●"

var (
	Title = lipgloss.NewStyle().Bold(true)

	// Active marks the profile governing the current directory.
	Active = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)

	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	Key = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	Warn = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	Bad = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	Good = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

// Table renders aligned columns without pulling in a table dependency.
func Table(w io.Writer, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && lipgloss.Width(cell) > widths[i] {
				widths[i] = lipgloss.Width(cell)
			}
		}
	}

	for _, row := range rows {
		var b strings.Builder
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(cell)+2))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}
