package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gronitab/git-release/internal/ui"
)

const primeContent = `# git-release — Agent Quick Reference

## What it does
git-release manages semver tags and release candidates on the current repo.
Tags follow the pattern: v1.2.3 (prod) and v1.2.3-rc / v1.2.3-rc.2 (RC).

## Read-only commands (safe to run any time)

  git release status        Show current state: latest prod, in-flight RC,
                            and what the next stage/release would create.

  git release ls            List all release tags descending, annotated with
                            [latest prod], [in-flight RC], [superseded RC].

## Mutating commands (fetch remote tag state first, push on success)

  git release stage         Continue whatever is in-flight (or start a patch RC).
  git release stage minor   Target the next minor version.
  git release stage major   Target the next major version.

  git release               Promote in-flight RC → prod, or create a direct patch
                            release if no RC is in flight.
  git release minor         Direct minor release (prompts if RC is in flight).
  git release major         Direct major release (prompts if RC is in flight).

## Flags (all mutating commands)

  -n, --dry-run   Print what would happen without pushing any tags.
  -y, --yes       Skip confirmation prompts (use in CI pipelines).

## Typical workflow

  git release status              # where are we?
  git release stage               # create v1.2.3-rc
  git release stage               # create v1.2.3-rc.2
  git release                     # promote → v1.2.3

## Rules to know

  - "stage" without a scope always continues whatever is in flight.
  - Staging a lower scope than the in-flight RC is an error.
  - "release" without a scope promotes the RC; never bypasses it silently.
  - Stale RCs are left in place (not deleted) when a higher-scope RC supersedes them.
  - All operations are idempotent on read; never force-push.
`

// primeOutput is kept for backwards compatibility (no-arg invocation).
const primeOutput = primeContent

const claudeSkillFrontmatter = `---
name: git-release
description: Manages semver tags and release candidates. Use when the user wants to stage, promote, or check the status of releases.
---
`

const primeSection = "<!-- git-release prime -->"

func newPrimeCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "prime [tool]",
		Short: "Print or install the agent quick-reference guide",
		Long: `Prints a concise reference of all git-release commands and rules,
intended to be injected into an AI agent's context.

Supported tools: claude, opencode, copilot, cursor, aider

Without a tool argument, prints the raw markdown to stdout.

With a tool argument:
  (no flag)  Install into the current project's config for that tool.
  --global   Install into the user's global config for that tool.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cmd.Print(primeOutput)
				return nil
			}
			p := &ui.Printer{Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()}
			return installPrime(p, args[0], global)
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Install into user's global config")
	return cmd
}

func installPrime(p *ui.Printer, tool string, global bool) error {
	switch strings.ToLower(tool) {
	case "claude":
		return installClaude(p, global)
	case "opencode":
		return installOpencode(p, global)
	case "copilot":
		return installCopilot(p, global)
	case "cursor":
		return installCursor(p, global)
	case "aider":
		return installAider(p, global)
	default:
		return fmt.Errorf("unknown tool %q; supported: claude, opencode, copilot, cursor, aider", tool)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func configDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".config")
}

// writeFile creates parent dirs and writes content to path.
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// appendSection appends content to path, creating it if needed.
// It is idempotent: if primeSection marker already exists, it replaces
// everything from the marker to the end with the new content.
func appendSection(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	marker := primeSection
	block := "\n" + marker + "\n" + content

	if idx := strings.Index(string(existing), marker); idx >= 0 {
		// Replace existing block
		return os.WriteFile(path, []byte(string(existing[:idx])+block), 0644)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(block)
	return err
}

// ── Claude ────────────────────────────────────────────────────────────────────

func installClaude(p *ui.Printer, global bool) error {
	if global {
		path := filepath.Join(homeDir(), ".claude", "skills", "git-release", "SKILL.md")
		if err := writeFile(path, claudeSkillFrontmatter+primeContent); err != nil {
			return err
		}
		p.Success("installed Claude Code skill → " + path)
		return nil
	}

	path := "CLAUDE.md"
	if err := appendSection(path, primeContent); err != nil {
		return err
	}
	p.Success("updated " + path)
	return nil
}

// ── OpenCode ──────────────────────────────────────────────────────────────────

func installOpencode(p *ui.Printer, global bool) error {
	var path string
	if global {
		path = filepath.Join(configDir(), "opencode", "AGENTS.md")
	} else {
		path = "AGENTS.md"
	}
	if err := appendSection(path, primeContent); err != nil {
		return err
	}
	p.Success("updated " + path)
	return nil
}

// ── Copilot ───────────────────────────────────────────────────────────────────

func installCopilot(p *ui.Printer, global bool) error {
	if global {
		p.Errorf("Copilot does not support a global instructions file; installing locally instead")
	}
	path := filepath.Join(".github", "copilot-instructions.md")
	if err := appendSection(path, primeContent); err != nil {
		return err
	}
	p.Success("updated " + path)
	return nil
}

// ── Cursor ────────────────────────────────────────────────────────────────────

func installCursor(p *ui.Printer, global bool) error {
	var path string
	if global {
		path = filepath.Join(homeDir(), ".cursor", "rules", "git-release.md")
	} else {
		path = filepath.Join(".cursor", "rules", "git-release.md")
	}
	if err := writeFile(path, primeContent); err != nil {
		return err
	}
	p.Success("written " + path)
	return nil
}

// ── Aider ─────────────────────────────────────────────────────────────────────

func installAider(p *ui.Printer, global bool) error {
	var (
		primePath string
		confPath  string
	)
	if global {
		primePath = filepath.Join(configDir(), "git-release", "prime.md")
		confPath = filepath.Join(homeDir(), ".aider.conf.yml")
	} else {
		primePath = ".git-release-prime.md"
		confPath = ".aider.conf.yml"
	}

	if err := writeFile(primePath, primeContent); err != nil {
		return err
	}

	if err := appendAiderRead(confPath, primePath); err != nil {
		return err
	}

	p.Success(fmt.Sprintf("written %s; added to %s read list", primePath, confPath))
	return nil
}

// appendAiderRead ensures confPath contains `read: [primePath]`.
// It parses the file line-by-line to avoid a YAML library dependency.
func appendAiderRead(confPath, primePath string) error {
	if err := os.MkdirAll(filepath.Dir(confPath), 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(confPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	entry := "  - " + primePath
	lines := strings.Split(string(data), "\n")

	// Check if already present
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(entry) {
			return nil
		}
	}

	// Find existing `read:` block and append inside it
	for i, l := range lines {
		if strings.TrimRight(l, " \t") == "read:" {
			lines = append(lines[:i+1], append([]string{entry}, lines[i+1:]...)...)
			return os.WriteFile(confPath, []byte(strings.Join(lines, "\n")), 0644)
		}
	}

	// No read: block — scan to find the right place or just append
	f, err := os.OpenFile(confPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	// Add a newline if the file doesn't end with one
	if len(data) > 0 && data[len(data)-1] != '\n' {
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "read:")
	fmt.Fprintln(w, entry)
	return w.Flush()
}
