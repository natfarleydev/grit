package cmd

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log [paths...]",
	Short: "Show commit log in git log style",
	Run:   runLog,
}

func init() {
	rootCmd.AddCommand(logCmd)
}

func runLog(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		args = []string{"./"}
	}

	for _, path := range args {
		repo, err := git.PlainOpen(path)
		if err != nil {
			fmt.Printf("Error opening repository at %s: %v\n", path, err)
			continue
		}

		head, err := repo.Head()
		if err != nil {
			fmt.Printf("Error getting HEAD for repository at %s: %v\n", path, err)
			continue
		}

		commits, err := repo.Log(&git.LogOptions{From: head.Hash()})
		if err != nil {
			fmt.Printf("Error getting commits for repository at %s: %v\n", path, err)
			continue
		}

		if len(args) > 1 {
			fmt.Printf("\nRepository: %s\n", path)
		}

		err = commits.ForEach(func(c *object.Commit) error {
			stats, err := c.Stats()
			if err != nil {
				return err
			}

			var added, deleted int
			for _, stat := range stats {
				added += stat.Addition
				deleted += stat.Deletion
			}

			fmt.Printf("\ncommit %s\n", c.Hash)
			fmt.Printf("Author: %s <%s>\n", c.Author.Name, c.Author.Email)
			fmt.Printf("Date:   %s\n", c.Author.When.Format(time.RFC3339))
			fmt.Printf("\n    %s\n", c.Message)
			fmt.Printf("\n    %d file(s) changed, %d insertion(s)(+), %d deletion(s)(-)\n", 
				len(stats), added, deleted)

			return nil
		})

		if err != nil {
			fmt.Printf("Error processing commits for repository at %s: %v\n", path, err)
		}
	}
}
