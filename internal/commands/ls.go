package commands

import (
	"github.com/spf13/cobra"
	"github.com/gronitab/git-release/internal/git"
	"github.com/gronitab/git-release/internal/semver"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all release tags",
		Long:  `Lists all release-related tags in descending semver order, annotated with commit hash, date, and status.`,
		Args:  cobra.NoArgs,
		RunE:  runLs,
	}
}

func runLs(cmd *cobra.Command, args []string) error {
	ctx, err := loadContext(cmd, false)
	if err != nil {
		cmd.PrintErrf("✗ %v\n", err)
		return err
	}

	// Sort tag infos by version descending
	semver.SortDesc(ctx.state.AllVersions)
	sorted := make([]git.TagInfo, 0, len(ctx.tags))
	for _, v := range ctx.state.AllVersions {
		for _, info := range ctx.tags {
			if info.Tag.Equal(v) {
				sorted = append(sorted, info)
				break
			}
		}
	}

	ctx.print.TagList(sorted, ctx.state, ctx.cfg.TagPrefix)
	return nil
}
