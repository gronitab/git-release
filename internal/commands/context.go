package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gronitab/git-release/internal/config"
	"github.com/gronitab/git-release/internal/git"
	"github.com/gronitab/git-release/internal/release"
	"github.com/gronitab/git-release/internal/semver"
	"github.com/gronitab/git-release/internal/ui"
)

// runContext holds everything resolved for a command invocation.
type runContext struct {
	dir    string
	cfg    *config.Config
	state  release.State
	tags   []git.TagInfo
	print  *ui.Printer
	dryRun bool
	yes    bool
}

// loadContext resolves the working directory, config, and remote tag state.
// It enforces release.requireClean and release.releaseBranch if set.
func loadContext(cmd *cobra.Command, mutating bool) (*runContext, error) {
	dir, err := repoRoot()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}

	p := &ui.Printer{Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()}
	ctx := &runContext{dir: dir, cfg: cfg, print: p, dryRun: flagDryRun, yes: flagYes}

	if mutating && cfg.RequireClean && !flagDryRun {
		clean, err := git.IsClean(dir)
		if err != nil {
			return nil, fmt.Errorf("checking working tree: %w", err)
		}
		if !clean {
			return nil, fmt.Errorf("working tree is dirty; commit or stash changes first (or set release.requireClean=false)")
		}
	}

	if mutating && cfg.ReleaseBranch != "" {
		branch, err := git.CurrentBranch(dir)
		if err != nil {
			return nil, fmt.Errorf("getting current branch: %w", err)
		}
		if branch != cfg.ReleaseBranch {
			return nil, fmt.Errorf("releases must be made from %q (currently on %q)", cfg.ReleaseBranch, branch)
		}
	}

	// Authoritative tag state: remote for mutating ops, local for read-only
	var versions []semver.Version
	if mutating && !flagDryRun {
		versions, err = git.ListRemoteTags(dir, cfg.Remote, cfg.TagPrefix)
		if err != nil {
			// Fall back to local with a warning
			p.Errorf("warning: could not fetch remote tags (%v); using local state", err)
			versions, err = git.ListLocalTags(dir, cfg.TagPrefix)
			if err != nil {
				return nil, err
			}
		}
	} else {
		versions, err = git.ListLocalTags(dir, cfg.TagPrefix)
		if err != nil {
			return nil, err
		}
	}

	ctx.state = release.Classify(versions)

	// Also load tag infos for ls/status
	ctx.tags, _ = git.ListLocalTagsWithInfo(dir, cfg.TagPrefix)

	return ctx, nil
}

// parseScope maps a string argument to a Scope constant.
func parseScope(arg string) (release.Scope, error) {
	switch strings.ToLower(arg) {
	case "", "patch":
		return release.ScopeNone, nil
	case "minor":
		return release.ScopeMinor, nil
	case "major":
		return release.ScopeMajor, nil
	default:
		return 0, fmt.Errorf("unknown scope %q — use patch, minor, or major", arg)
	}
}

// repoRoot returns the root directory of the git repo containing the CWD.
func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	c := exec.Command("git", "rev-parse", "--show-toplevel")
	c.Dir = cwd
	out, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// confirm prompts the user to type expectedValue to proceed.
// Returns true if confirmed, false if aborted.
// If yes flag is set, always confirms.
func confirm(ctx *runContext, prompt, expectedValue string) (bool, error) {
	if ctx.yes {
		return true, nil
	}

	// Read from /dev/tty so the command is safe in pipelines
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return false, fmt.Errorf("cannot open /dev/tty: %w", err)
	}
	defer tty.Close()

	fmt.Fprintf(ctx.print.Out, "%s\nType %s to confirm, or press Enter to cancel: ", prompt, ui.StyleBold(expectedValue))

	var input string
	buf := make([]byte, 256)
	n, _ := tty.Read(buf)
	input = strings.TrimSpace(string(buf[:n]))

	return input == expectedValue, nil
}

// pushReleaseBranch pushes the release branch to the remote (fast-forward only)
// so the commits the tag will point at land on the branch BEFORE the tag is
// pushed — the fix for a release leaving the remote branch behind its own tag
// (bridge grit-4h2uo). No-op under --dry-run or release.pushBranch=false. The
// branch is release.releaseBranch when set, else the current branch.
func pushReleaseBranch(ctx *runContext) error {
	if !ctx.cfg.PushBranch || ctx.dryRun {
		return nil
	}
	branch := ctx.cfg.ReleaseBranch
	if branch == "" {
		b, err := git.CurrentBranch(ctx.dir)
		if err != nil {
			return fmt.Errorf("resolving current branch: %w", err)
		}
		branch = b
	}
	return git.PushBranch(ctx.dir, ctx.cfg.Remote, branch, false)
}
