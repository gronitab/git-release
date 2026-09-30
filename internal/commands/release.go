package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gronitab/git-release/internal/git"
	"github.com/gronitab/git-release/internal/release"
	"github.com/gronitab/git-release/internal/semver"
	"github.com/gronitab/git-release/internal/ui"
)

func runRelease(cmd *cobra.Command, args []string) error {
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

	target, err := release.NextRelease(ctx.state, scope)
	if errors.Is(err, release.ErrRCInFlight) {
		rc := ctx.state.InFlightRC
		prompt := fmt.Sprintf(
			"Warning: %s is in flight.\nReleasing %s directly will bypass it.",
			ui.FormatTag(*rc),
			ui.FormatTag(target),
		)
		// Compute the target version manually since NextRelease errored
		target, _ = releaseTarget(ctx.state, scope)
		confirmed, confirmErr := confirm(ctx, prompt, target.String(ctx.cfg.TagPrefix))
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			ctx.print.Error("aborted")
			return errAbort
		}
	} else if err != nil {
		ctx.print.Errorf("%v", err)
		return err
	}

	tagName := target.String(ctx.cfg.TagPrefix)

	// Determine the ref to tag
	ref := "HEAD"
	if ctx.state.InFlightRC != nil && scope == release.ScopeNone {
		// Promoting: tag the RC's commit, not HEAD
		rcTag := ctx.state.InFlightRC.String(ctx.cfg.TagPrefix)
		resolved, resolveErr := git.ResolveRef(ctx.dir, rcTag)
		if resolveErr != nil {
			return fmt.Errorf("resolving RC tag %s: %w", rcTag, resolveErr)
		}
		ref = resolved
	}

	if ctx.dryRun {
		ctx.print.DryRun(fmt.Sprintf("would create and push tag %s → %s", tagName, ref))
		return nil
	}

	// Push the branch before the tag so origin never lags its own tag (grit-4h2uo).
	if err := pushReleaseBranch(ctx); err != nil {
		ctx.print.Errorf("failed to push release branch (refusing to tag ahead of it): %v", err)
		return err
	}

	if err := git.CreateAndPushTag(ctx.dir, ctx.cfg.Remote, tagName, ref, ctx.cfg.SignTags, false); err != nil {
		ctx.print.Errorf("failed to create/push tag: %v", err)
		return err
	}

	ctx.print.TagCreated(tagName, ctx.cfg.TagPrefix, false)
	return nil
}

// releaseTarget computes the direct release target version (bypassing any RC).
func releaseTarget(s release.State, scope release.Scope) (semver.Version, error) {
	switch scope {
	case release.ScopeMinor:
		return s.LatestProd.BumpMinor(), nil
	case release.ScopeMajor:
		return s.LatestProd.BumpMajor(), nil
	default:
		return s.LatestProd.BumpPatch(), nil
	}
}
