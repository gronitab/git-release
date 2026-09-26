// Package ui provides terminal output styling for git-release.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/skaramicke/git-release/internal/git"
	"github.com/skaramicke/git-release/internal/release"
	"github.com/skaramicke/git-release/internal/semver"
)

var (
	colorProd    = lipgloss.Color("#34D399") // emerald
	colorRC      = lipgloss.Color("#FBBF24") // amber
	colorStale   = lipgloss.Color("#6B7280") // gray
	colorMuted   = lipgloss.Color("#9CA3AF") // light gray
	colorError   = lipgloss.Color("#F87171") // red
	colorSuccess = lipgloss.Color("#34D399") // emerald
	colorHint    = lipgloss.Color("#60A5FA") // blue

	styleProd    = lipgloss.NewStyle().Foreground(colorProd).Bold(true)
	styleRC      = lipgloss.NewStyle().Foreground(colorRC).Bold(true)
	styleStale   = lipgloss.NewStyle().Foreground(colorStale)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)
	styleError   = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	styleSuccess = lipgloss.NewStyle().Foreground(colorSuccess)
	styleHint    = lipgloss.NewStyle().Foreground(colorHint)
	styleBold    = lipgloss.NewStyle().Bold(true)
)

// Printer writes styled output to a writer (defaults to os.Stdout/Stderr).
type Printer struct {
	Out io.Writer
	Err io.Writer
}

// Default returns a Printer writing to stdout/stderr.
func Default() *Printer {
	return &Printer{Out: os.Stdout, Err: os.Stderr}
}

func (p *Printer) out(format string, args ...any) {
	fmt.Fprintf(p.Out, format+"\n", args...)
}

func (p *Printer) err(format string, args ...any) {
	fmt.Fprintf(p.Err, format+"\n", args...)
}

// Success prints a green success message with a checkmark.
func (p *Printer) Success(msg string) {
	p.out("%s %s", styleSuccess.Render("✓"), msg)
}

// DryRun prints a blue message indicating what would happen.
func (p *Printer) DryRun(msg string) {
	p.out("%s %s", styleHint.Render("~"), styleMuted.Render("[dry-run]")+" "+msg)
}

// Error prints a red error message to stderr.
func (p *Printer) Error(msg string) {
	p.err("%s %s", styleError.Render("✗"), msg)
}

// Errorf prints a formatted red error to stderr.
func (p *Printer) Errorf(format string, args ...any) {
	p.Error(fmt.Sprintf(format, args...))
}

// TagCreated prints the "created and pushed" success line.
func (p *Printer) TagCreated(tag, prefix string, dryRun bool) {
	v, _ := semver.ParseTag(tag, prefix)
	label := FormatTag(v)
	if dryRun {
		p.DryRun(fmt.Sprintf("would create and push %s", label))
	} else {
		p.out("%s created and pushed %s", styleSuccess.Render("✓"), label)
	}
}

// Status prints the full release status summary.
func (p *Printer) Status(s release.State, prefix string) {
	latestProd := styleMuted.Render("none")
	if !s.LatestProd.Equal(semver.Version{}) {
		latestProd = styleProd.Render(s.LatestProd.String(prefix))
	}

	inflight := styleMuted.Render("none")
	if s.InFlightRC != nil {
		inflight = styleRC.Render(s.InFlightRC.String(prefix))
	}

	nextStageStr := styleMuted.Render("—")
	if ns, err := release.NextStage(s, release.ScopeNone); err == nil {
		nextStageStr = styleHint.Render(ns.String(prefix)) +
			styleMuted.Render("  (git release stage)")
	}

	nextReleaseStr := styleMuted.Render("—")
	if nr, err := release.NextRelease(s, release.ScopeNone); err == nil {
		nextReleaseStr = styleHint.Render(nr.String(prefix)) +
			styleMuted.Render("  (git release)")
	}

	labelW := 16
	row := func(label, value string) string {
		pad := strings.Repeat(" ", labelW-len(label))
		return styleBold.Render(label+":") + pad + value
	}

	p.out("")
	p.out(row("Latest prod", latestProd))
	p.out(row("In-flight RC", inflight))
	p.out(row("Next stage", nextStageStr))
	p.out(row("Next release", nextReleaseStr))

	// The CI verdict of the in-flight RC (GRIT-6872): appended separately by
	// the status command so the git-only summary stays complete on its own.
}

// CINone prints the muted line for an in-flight RC with no reachable run.
func (p *Printer) CINone(tag string) {
	p.out(styleBold.Render("RC CI:") + strings.Repeat(" ", 10) +
		styleMuted.Render("no run found for "+tag+" (no gh, or nothing triggered)"))
}

// CIVerdict prints the release job's state next to the in-flight RC tag.
func (p *Printer) CIVerdict(tag string, runID int64, verdict string) {
	label := styleBold.Render("RC CI:") + strings.Repeat(" ", 10)
	if verdict == "" {
		verdict = "run state unknown"
	}
	var styledVerdict string
	if strings.HasPrefix(verdict, "FAILED") || strings.HasPrefix(verdict, "CANCELLED") {
		styledVerdict = styleError.Render(verdict)
	} else if strings.HasPrefix(verdict, "succeeded") {
		styledVerdict = styleSuccess.Render(verdict)
	} else {
		styledVerdict = styleHint.Render(verdict)
	}
	p.out(label + styleRC.Render(tag) + "  →  " + styledVerdict +
		styleMuted.Render(fmt.Sprintf("  (grit ci log --run %d)", runID)))
}

// TagList prints the annotated tag list.
func (p *Printer) TagList(tags []git.TagInfo, state release.State, prefix string) {
	if len(tags) == 0 {
		p.out(styleMuted.Render("  no release tags found"))
		return
	}

	// Column widths
	maxTag := 0
	for _, t := range tags {
		if l := len(t.Tag.String(prefix)); l > maxTag {
			maxTag = l
		}
	}

	p.out("")
	for _, info := range tags {
		tagStr := info.Tag.String(prefix)
		pad := strings.Repeat(" ", maxTag-len(tagStr)+2)

		var styled, badge string
		switch {
		case state.InFlightRC != nil && info.Tag.Equal(*state.InFlightRC):
			styled = styleRC.Render(tagStr)
			badge = styleRC.Render("[in-flight RC]")
		case !info.Tag.IsRC && info.Tag.Equal(state.LatestProd):
			styled = styleProd.Render(tagStr)
			badge = styleProd.Render("[latest prod]")
		case info.Tag.IsRC:
			styled = styleStale.Render(tagStr)
			badge = styleStale.Render("[superseded RC]")
		default:
			styled = styleMuted.Render(tagStr)
			badge = ""
		}

		date := styleMuted.Render(info.Date.Format("2006-01-02"))
		hash := styleMuted.Render(info.Hash)

		line := fmt.Sprintf("  %s%s%s  %s  %s", styled, pad, hash, date, badge)
		p.out(strings.TrimRight(line, " "))
	}
	p.out("")
}

// FormatTag returns a styled inline version string.
func FormatTag(v semver.Version) string {
	if v.IsRC {
		return styleRC.Render(v.String("v"))
	}
	return styleProd.Render(v.String("v"))
}

// StyleBold renders text in bold.
func StyleBold(s string) string {
	return styleBold.Render(s)
}

// HumanAge formats a time as a human-readable "N days ago" string.
func HumanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
}
