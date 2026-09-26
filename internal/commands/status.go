package commands

import (
	"time"

	"github.com/skaramicke/git-release/internal/ci"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current release state",
		Long:  `Prints a summary of the current release state: latest prod tag, in-flight RC, and what the next stage and release would be.`,
		Args:  cobra.NoArgs,
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx, err := loadContext(cmd, false)
	if err != nil {
		cmd.PrintErrf("✗ %v\n", err)
		return err
	}

	ctx.print.Status(ctx.state, ctx.cfg.TagPrefix)

	// THE VERDICT NEXT TO THE TAG (GRIT-6872). An in-flight RC whose release
	// job failed is not in flight — it is finished and lost, and status is
	// where whoever cut it comes to ask. Best-effort: no gh (pods) or no run
	// degrades to one muted line, never an error.
	if ctx.state.InFlightRC == nil {
		return nil
	}
	tag := ctx.state.InFlightRC.String(ctx.cfg.TagPrefix)
	r, _ := ci.Probe(ctx.dir, tag)
	if r == nil {
		ctx.print.CINone(tag)
		return nil
	}
	jobs, _ := ci.Jobs(ctx.dir, r.DatabaseID)
	ctx.print.CIVerdict(tag, r.DatabaseID, ci.Verdict(r, jobs, time.Now()))
	return nil
}
