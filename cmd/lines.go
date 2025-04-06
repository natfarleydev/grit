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
	weekToDate     bool
	noCache        bool
	timeNow        = time.Now // For testing
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
	linesCmd.Flags().BoolVarP(&weekToDate, "week-to-date", "w", false, "Count lines from start of current week (Monday) instead of current day")
	linesCmd.Flags().BoolVarP(&noCache, "no-cache", "n", false, "Disable caching of results")
}

func runLines(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		args = []string{"./"}
	}

	var cache *Cache
	var err error

	if !noCache {
		// Try to load cache
		cache, err = loadCache()
		if err != nil {
			fmt.Printf("Warning: Could not load cache: %v\n", err)
			cache = &Cache{Entries: make([]CacheEntry, 0)}
		}

		// Try to find matching cache entry
		entry := findMatchingCacheEntry(cache, args, authorRegex, remoteName, filenamesRegex, weekToDate)
		if entry != nil && isCacheValid(entry, args) {
			fmt.Printf("+%d/-%d", entry.Results.Added, entry.Results.Deleted)
			return
		}
	}

	var filenameRe *regexp.Regexp
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
	headHashes := make(map[string]string)

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
			headHashes[path] = hash.String()
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
			headHashes[path] = hash.String()
		}

		now := timeNow()
		var startTime time.Time
		if weekToDate {
			// Calculate start of current week (Monday)
			weekday := now.Weekday()
			var daysToSubtract int
			if weekday == time.Sunday {
				daysToSubtract = 7 // Go back 7 days to get to last Monday
			} else {
				daysToSubtract = int(weekday) - 1
			}
			startTime = now.AddDate(0, 0, -daysToSubtract)
			// Set to start of Monday (00:00:00) to include all commits from Monday onwards
			startTime = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, now.Location())
		} else {
			// Start of current day
			startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		}

		commits, err := repo.Log(&git.LogOptions{From: hash})
		if err != nil {
			fmt.Printf("Error getting commits for repository at %s: %v\n", path, err)
			continue
		}

		err = commits.ForEach(func(c *object.Commit) error {
			if c.Author.When.Before(startTime) {
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

	// Create new cache entry and update cache if caching is enabled
	if !noCache {
		newEntry := CacheEntry{
			Args: struct {
				AuthorRegex    string
				RemoteName     string
				FilenamesRegex string
				WeekToDate     bool
			}{
				AuthorRegex:    authorRegex,
				RemoteName:     remoteName,
				FilenamesRegex: filenamesRegex,
				WeekToDate:     weekToDate,
			},
			Paths:      args,
			HeadHashes: headHashes,
			Results: struct {
				Added   int64
				Deleted int64
			}{
				Added:   totalAdded,
				Deleted: totalDeleted,
			},
			Timestamp: time.Now(),
		}

		// Update cache
		cache.Entries = append(cache.Entries, newEntry)
		if err := saveCache(cache); err != nil {
			fmt.Printf("Warning: Could not save cache: %v\n", err)
		}
	}

	fmt.Printf("+%d/-%d", totalAdded, totalDeleted)
}
