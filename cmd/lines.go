package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/spf13/cobra"
)

var (
	authorRegex    string
	remoteName     string
	filenamesRegex string
	linesCmd       = &cobra.Command{
		Use:   "lines [paths...]",
		Short: "Count lines added/removed by authors matching regex",
		Run:   runLines,
	}
)

func init() {
	countCmd.AddCommand(linesCmd)
	linesCmd.Flags().StringVarP(&authorRegex, "author-regex", "a", "", "Regex pattern to match author name or email")
	linesCmd.Flags().StringVarP(&remoteName, "remote", "r", "", "Remote name to use for branch references")
	linesCmd.Flags().StringVarP(&filenamesRegex, "filenames-regex", "f", "", "Regex pattern to match filenames (e.g., '(py$|yml$)' for Python and YAML files)")
}

func runLines(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		args = []string{"./"}
	}

	var filenameRe *regexp.Regexp
	var err error

	if filenamesRegex != "" {
		filenameRe, err = regexp.Compile(filenamesRegex)
		if err != nil {
			fmt.Printf("Error compiling filename regex pattern: %v\n", err)
			return
		}
	}

	re, err := regexp.Compile(authorRegex)
	if err != nil {
		fmt.Printf("Error compiling author regex pattern: %v\n", err)
		return
	}

	var totalAdded, totalDeleted int64

	for _, pathSpec := range args {
		// Split path and branch if specified (path@branch)
		path := pathSpec
		branch := ""
		if idx := strings.LastIndex(pathSpec, "@"); idx != -1 {
			path = pathSpec[:idx]
			branch = pathSpec[idx+1:]
		}

		repo, err := git.PlainOpen(path)
		if err != nil {
			fmt.Printf("Error opening repository at %s: %v\n", path, err)
			continue
		}

		var hash plumbing.Hash
		if branch == "" {
			// Use HEAD if no branch specified
			head, err := repo.Head()
			if err != nil {
				fmt.Printf("Error getting HEAD for repository at %s: %v\n", path, err)
				continue
			}
			hash = head.Hash()
		} else {
			var refName plumbing.ReferenceName
			if remoteName != "" {
				// Check remote branch
				refName = plumbing.NewRemoteReferenceName(remoteName, branch)
			} else {
				// Check local branch
				refName = plumbing.NewBranchReferenceName(branch)
			}
			branchRef, err := repo.Reference(refName, true)
			if err != nil {
				fmt.Printf("Error getting branch %s for repository at %s: %v\n", branch, path, err)
				continue
			}
			hash = branchRef.Hash()
		}

		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		commits, err := repo.Log(&git.LogOptions{From: hash})
		if err != nil {
			fmt.Printf("Error getting commits for repository at %s: %v\n", path, err)
			continue
		}

		err = commits.ForEach(func(c *object.Commit) error {
			if c.Author.When.Before(startOfDay) {
				return nil
			}

			// If the commit is a merge commit, skip it
			if len(c.ParentHashes) > 1 {
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
				// Filter by filename regex if specified
				if filenameRe != nil && !filenameRe.MatchString(stat.Name) {
					continue
				}
				totalAdded += int64(stat.Addition)
				totalDeleted += int64(stat.Deletion)
			}
			return nil
		})

		if err != nil {
			fmt.Printf("Error processing commits for repository at %s: %v\n", path, err)
			continue
		}
	}

	fmt.Printf("+%d/-%d", totalAdded, totalDeleted)
}
