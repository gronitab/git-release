package commands

import (
	"fmt"
	"time"

	"github.com/skaramicke/git-release/internal/ci"
	"github.com/skaramicke/git-release/internal/git"
	"github.com/skaramicke/git-release/internal/release"
	"github.com/spf13/cobra"
)

func newStageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stage [scope]",
		Short: "Create or increment a release candidate",
		Long: `Creates or increments a release candidate tag.

Without a scope, continues whatever is already in flight (or starts a patch RC).
With a scope (minor, major), applies the requested bump to the latest prod tag.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runStage,
		Example: `  git release stage           # continue in-flight RC, or create patch-rc
  git release stage minor     # target next minor version
  git release stage major     # target next major version
  git release stage --dry-run # preview without pushing`,
	}
}

func runStage(cmd *cobra.Command, args []string) error {
	scope := release.ScopeNone
	if len(args) == 1 {
		var err error
		scope, err = parseScope(args[0])
		if err != nil {
			return err
		}
	}

	ctx, err := loadContext(cmd, true)
	if err != nil {
		cmd.PrintErrf("✗ %v\n", err)
		return err
	}

	next, err := release.NextStage(ctx.state, scope)
	if err != nil {
		ctx.print.Errorf("%v", err)
		return err
	}

	tagName := next.String(ctx.cfg.TagPrefix)

	if ctx.dryRun {
		ctx.print.DryRun(fmt.Sprintf("would create and push tag %s → HEAD", tagName))
		// Show stale note if applicable
		if ctx.state.InFlightRC != nil && scope != release.ScopeNone {
			old := ctx.state.InFlightRC.String(ctx.cfg.TagPrefix)
			if old != tagName {
				ctx.print.DryRun(fmt.Sprintf("note: %s would become stale (not deleted)", old))
			}
		}
		return nil
	}

	// Atomicity guard: if HEAD is already tagged by the in-flight RC, refuse
	// rather than push a new RC tag at the same commit. Catches the race
	// where two `git release stage` callers on the same HEAD would otherwise
	// produce consecutive RC tags pointing at one source commit.
	if err := refuseIfHeadAlreadyStaged(ctx); err != nil {
		ctx.print.Errorf("%v", err)
		return err
	}

	// Push the branch BEFORE the tag so origin never lags its own tag — abort
	// (no tag created) if the branch can't fast-forward (grit-4h2uo).
	if err := pushReleaseBranch(ctx); err != nil {
		ctx.print.Errorf("failed to push release branch (refusing to tag ahead of it): %v", err)
		return err
	}

	if err := git.CreateAndPushTag(ctx.dir, ctx.cfg.Remote, tagName, "HEAD", ctx.cfg.SignTags, false); err != nil {
		ctx.print.Errorf("failed to create/push tag: %v", err)
		return err
	}

	ctx.print.TagCreated(tagName, ctx.cfg.TagPrefix, false)

	// HAND BACK THE RUN (GRIT-6872). "created and pushed" is true about the
	// tag and says nothing about the release: the job it triggers can fail
	// minutes later while the caller waits for a rollout that is not coming.
	// Best-effort — no gh, no run, nothing changes about the exit code.
	if r, _ := ci.WaitForRun(ctx.dir, tagName, 3, 3*time.Second); r != nil {
		ctx.print.Success(fmt.Sprintf("release job started: %s (grit ci log --run %d)", r.HTMLURL, r.DatabaseID))
	} else {
		ctx.print.Success(fmt.Sprintf("no release run appeared for %s yet — check with: gh run list --branch %s", tagName, tagName))
	}

	// Note stale old RC
	if ctx.state.InFlightRC != nil {
		old := ctx.state.InFlightRC.String(ctx.cfg.TagPrefix)
		if old != tagName {
			ctx.print.Success(fmt.Sprintf("note: %s is now stale (not deleted)", old))
		}
	}

	return nil
}

// refuseIfHeadAlreadyStaged returns an error if the in-flight RC already
// points at HEAD on the remote. A transient remote-read failure is
// non-fatal — the subsequent push will surface real errors.
func refuseIfHeadAlreadyStaged(ctx *runContext) error {
	if ctx.state.InFlightRC == nil {
		return nil
	}
	inFlightTag := ctx.state.InFlightRC.String(ctx.cfg.TagPrefix)
	inFlightCommit, err := git.ResolveRemoteRef(ctx.dir, ctx.cfg.Remote, "refs/tags/"+inFlightTag)
	if err != nil {
		return nil
	}
	headCommit, err := git.ResolveRef(ctx.dir, "HEAD")
	if err != nil {
		return fmt.Errorf("resolving HEAD: %w", err)
	}
	if inFlightCommit != headCommit {
		return nil
	}
	short := headCommit
	if len(short) > 7 {
		short = short[:7]
	}
	return fmt.Errorf("HEAD (%s) is already tagged as %s; commit something or run `git release` to promote", short, inFlightTag)
}
