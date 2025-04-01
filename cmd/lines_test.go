package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func setupTestRepo(t *testing.T) (string, func()) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "grit-test")
	assert.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(dir, false)
	assert.NoError(t, err)

	// Create test files with different extensions
	files := map[string]string{
		"test.txt":   "line1\nline2\nline3",
		"config.yml": "name: test\nversion: 1.0\n",
		"script.py":  "def hello():\n    print('hello')\n",
	}

	// Setup the worktree
	worktree, err := repo.Worktree()
	assert.NoError(t, err)

	// Add and commit initial files
	for name, content := range files {
		path := filepath.Join(dir, name)
		err = ioutil.WriteFile(path, []byte(content), 0644)
		assert.NoError(t, err)
		_, err = worktree.Add(name)
		assert.NoError(t, err)
	}

	// Use a fixed reference time: Monday, January 1, 2024 at 12:00:00 UTC
	referenceTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// First commit by Nathanael (early Monday)
	mainCommit, err := worktree.Commit("First commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Nathanael Farley",
			Email: "nathanael@example.com",
			When:  referenceTime.Add(-6 * time.Hour), // 6:00 AM Monday
		},
	})
	assert.NoError(t, err)

	// Create and switch to feature branch
	err = worktree.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.NewBranchReferenceName("feature"),
	})
	assert.NoError(t, err)

	// Modify files in feature branch - Mirabel's changes
	changes := map[string]string{
		"test.txt":  "line1\nline2 modified\nline3",
		"script.py": "def hello():\n    print('hello world')\n    return True\n",
	}
	for name, content := range changes {
		path := filepath.Join(dir, name)
		err = ioutil.WriteFile(path, []byte(content), 0644)
		assert.NoError(t, err)
		_, err = worktree.Add(name)
		assert.NoError(t, err)
	}

	_, err = worktree.Commit("Feature branch commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Mirabel Smith",
			Email: "mirabel@example.com",
			When:  referenceTime,
		},
	})
	assert.NoError(t, err)

	// Switch back to main and make another change
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: mainCommit,
	})
	assert.NoError(t, err)

	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
		Create: true,
	})
	assert.NoError(t, err)

	// Modify files in main branch - Nathanael's changes
	changes = map[string]string{
		"test.txt":   "line1\nline2\nline3 modified",
		"config.yml": "name: test\nversion: 2.0\nenv: prod\n",
	}
	for name, content := range changes {
		path := filepath.Join(dir, name)
		err = ioutil.WriteFile(path, []byte(content), 0644)
		assert.NoError(t, err)
		_, err = worktree.Add(name)
		assert.NoError(t, err)
	}

	_, err = worktree.Commit("Main branch commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Nathanael Farley",
			Email: "nathanael@example.com",
			When:  referenceTime.Add(2 * time.Hour), // 2:00 PM Monday
		},
	})
	assert.NoError(t, err)

	return dir, func() {
		os.RemoveAll(dir)
	}
}

func TestRunLines(t *testing.T) {
	// Mock time.Now() to return our fixed reference time
	referenceTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time {
		return referenceTime
	}
	defer func() {
		timeNow = time.Now
	}()

	// Create test repos
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	dir2, cleanup2 := setupTestRepoWithDifferentDays(t)
	defer cleanup2()

	tests := []struct {
		name           string
		authorRegex    string
		filenamesRegex string
		remote         string
		weekToDate     bool
		paths          []string
		wantAdded      int64
		wantDeleted    int64
		wantErr        bool
	}{
		{
			name:        "Match Nathanael on main branch",
			authorRegex: "Nathanael",
			paths:       []string{"./@main"},
			wantAdded:   10, // All changes: initial commit (7 lines) + second commit (3 lines)
			wantDeleted: 2,
		},
		{
			name:        "Match Mirabel on feature branch",
			authorRegex: "Mirabel",
			paths:       []string{"./@feature"},
			wantAdded:   3, // Modified one line in test.txt and added one line in script.py
			wantDeleted: 2,
		},
		{
			name:           "Match Python files only",
			filenamesRegex: "\\.py$",
			paths:          []string{"./@feature"},
			wantAdded:      4, // script.py changes: 2 lines from initial commit + 2 lines from feature branch
			wantDeleted:    1,
		},
		{
			name:           "Match YAML files only",
			filenamesRegex: "\\.ya?ml$",
			paths:          []string{"./@main"},
			wantAdded:      4, // config.yml changes: 2 lines from initial commit + 2 lines from main branch
			wantDeleted:    1,
		},
		{
			name:           "Match Python and YAML files",
			filenamesRegex: "(\\.py$|\\.ya?ml$)",
			paths:          []string{"./@main", "./@feature"},
			wantAdded:      12, // All Python and YAML changes: 4 from script.py + 4 from config.yml + 4 from feature branch
			wantDeleted:    2,
		},
		{
			name:           "Match Nathanael's YAML changes",
			authorRegex:    "Nathanael",
			filenamesRegex: "\\.ya?ml$",
			paths:          []string{"./@main"},
			wantAdded:      4, // config.yml changes: 2 lines from initial commit + 2 lines from main branch
			wantDeleted:    1,
		},
		{
			name:           "Invalid filename regex",
			filenamesRegex: "(",
			paths:          []string{"./@main"},
			wantErr:        true,
		},
		{
			name:        "Match no authors",
			authorRegex: "NonExistent",
			paths:       []string{"./@main", "./@feature"},
			wantAdded:   0,
			wantDeleted: 0,
		},
		{
			name:        "Week to date - Nathanael on main branch",
			authorRegex: "Nathanael",
			weekToDate:  true,
			paths:       []string{"./@main"},
			wantAdded:   10, // All changes from start of week: initial commit (7 lines) + second commit (3 lines)
			wantDeleted: 2,
		},
		{
			name:        "Week to date - Mirabel on feature branch",
			authorRegex: "Mirabel",
			weekToDate:  true,
			paths:       []string{"./@feature"},
			wantAdded:   3, // Only one commit in the week
			wantDeleted: 2,
		},
		{
			name:        "Week to date - commits from previous week",
			authorRegex: "Nathanael",
			weekToDate:  true,
			paths:       []string{"./@main"},
			wantAdded:   10, // All changes from start of week: initial commit (7 lines) + second commit (3 lines)
			wantDeleted: 2,
		},
		{
			name:        "Week to date - commits from previous week and current week",
			authorRegex: "Nathanael",
			weekToDate:  true,
			paths:       []string{"./@main"},
			wantAdded:   10, // All changes from start of week: initial commit (7 lines) + second commit (3 lines)
			wantDeleted: 2,
		},
		{
			name:        "Week to date - commits from current week only",
			authorRegex: "Mirabel",
			weekToDate:  true,
			paths:       []string{"./@feature"},
			wantAdded:   3, // Only one commit in the week
			wantDeleted: 2,
		},
		{
			name:        "Week to date - commits on Monday",
			authorRegex: "Monday",
			weekToDate:  true,
			paths:       []string{dir2},
			wantAdded:   5, // Initial commit with 5 lines
			wantDeleted: 0,
		},
		{
			name:        "Week to date - commits on Wednesday",
			authorRegex: "Wednesday",
			weekToDate:  true,
			paths:       []string{dir2},
			wantAdded:   3, // Modified first line and line2, line3 unchanged, line4 unchanged
			wantDeleted: 4, // Removed line5
		},
		{
			name:        "Week to date - commits on Sunday",
			authorRegex: "Sunday",
			weekToDate:  true,
			paths:       []string{dir2},
			wantAdded:   2, // Modified first line and line2 unchanged
			wantDeleted: 3, // Removed line4
		},
		{
			name:        "Week to date - all commits in week",
			authorRegex: "Monday|Wednesday|Sunday",
			weekToDate:  true,
			paths:       []string{dir2},
			wantAdded:   10, // All changes in the week
			wantDeleted: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Set the flags
			authorRegex = tt.authorRegex
			remoteName = tt.remote
			filenamesRegex = tt.filenamesRegex
			weekToDate = tt.weekToDate

			// Change paths to use the temp directory
			paths := make([]string, len(tt.paths))
			for i, p := range tt.paths {
				if p == dir2 {
					paths[i] = p
				} else {
					paths[i] = filepath.Join(dir, p)
				}
			}

			// Run the command
			runLines(nil, paths)

			// Restore stdout
			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)

			if tt.wantErr {
				assert.Contains(t, buf.String(), "Error compiling filename regex")
			} else {
				// Check output format
				expected := fmt.Sprintf("+%d/-%d", tt.wantAdded, tt.wantDeleted)
				assert.Equal(t, expected, buf.String())
			}
		})
	}
}

// setupTestRepoWithDifferentDays creates a test repo with commits on different days of the week
func setupTestRepoWithDifferentDays(t *testing.T) (string, func()) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "grit-test-different-days")
	assert.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(dir, false)
	assert.NoError(t, err)

	// Setup the worktree
	worktree, err := repo.Worktree()
	assert.NoError(t, err)

	// Use a fixed reference time: Monday, January 1, 2024 at 12:00:00 UTC
	referenceTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Create and commit initial file
	testFile := filepath.Join(dir, "test.txt")
	err = ioutil.WriteFile(testFile, []byte("initial content\nline2\nline3\nline4\nline5"), 0644)
	assert.NoError(t, err)
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

	// Initial commit on Monday
	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Monday",
			Email: "monday@example.com",
			When:  referenceTime,
		},
	})
	assert.NoError(t, err)

	// Modify and commit on Wednesday
	err = ioutil.WriteFile(testFile, []byte("modified on wednesday\nline2 changed\nline3\nline4"), 0644)
	assert.NoError(t, err)
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

	_, err = worktree.Commit("Wednesday commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Wednesday",
			Email: "wednesday@example.com",
			When:  referenceTime.Add(2 * 24 * time.Hour),
		},
	})
	assert.NoError(t, err)

	// Modify and commit on Sunday
	err = ioutil.WriteFile(testFile, []byte("modified on sunday\nline2 changed\nline3"), 0644)
	assert.NoError(t, err)
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

	_, err = worktree.Commit("Sunday commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Sunday",
			Email: "sunday@example.com",
			When:  referenceTime.Add(6 * 24 * time.Hour),
		},
	})
	assert.NoError(t, err)

	return dir, func() {
		os.RemoveAll(dir)
	}
}
