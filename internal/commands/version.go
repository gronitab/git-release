package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gronitab/git-release/internal/build"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "git-release %s (commit: %s, built: %s)\n",
				build.Version, build.Commit, build.Date)
		},
	}
}
