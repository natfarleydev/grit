package cmd

import (
	"github.com/spf13/cobra"
)

var countCmd = &cobra.Command{
	Use:   "count",
	Short: "Count various metrics in git repositories",
}

func init() {
	rootCmd.AddCommand(countCmd)
}
