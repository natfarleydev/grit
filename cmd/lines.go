package cmd

import (
	"fmt"
	"regexp"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
)

var (
	authorRegex string
	linesCmd    = &cobra.Command{
		Use:   "lines [paths...]",
		Short: "Count lines added/removed by authors matching regex",
		Run:   runLines,
	}
)

func init() {
	countCmd.AddCommand(linesCmd)
	linesCmd.Flags().StringVarP(&authorRegex, "author-regex", "a", "", "Regex pattern to match author name or email")
}

func runLines(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		args = []string{"./"}
	}

	re, err := regexp.Compile(authorRegex)
	if err != nil {
		fmt.Printf("Error compiling regex pattern: %v\n", err)
		return
	}

	var totalAdded, totalDeleted int64

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

		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		commits, err := repo.Log(&git.LogOptions{From: head.Hash()})
		if err != nil {
			fmt.Printf("Error getting commits for repository at %s: %v\n", path, err)
			continue
		}

		err = commits.ForEach(func(c *object.Commit) error {
			if c.Author.When.Before(startOfDay) {
				return nil
			}

			// Match against both name and email
			if !re.MatchString(c.Author.Name) && !re.MatchString(c.Author.Email) {
				return nil
			}

			stats, err := c.Stats()
			if err != nil {
				return err
			}

			for _, stat := range stats {
				totalAdded += int64(stat.Addition)
				totalDeleted += int64(stat.Deletion)
			}
			return nil
		})

		if err != nil {
			fmt.Printf("Error processing commits for repository at %s: %v\n", path, err)
		}
	}

	fmt.Printf("+%d/-%d", totalAdded, totalDeleted)
}
