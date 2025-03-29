package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "grit",
	Short: "Git repository inspection tool",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}
