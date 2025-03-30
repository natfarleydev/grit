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
		"test.txt":    "line1\nline2\nline3",
		"config.yml":  "name: test\nversion: 1.0\n",
		"script.py":   "def hello():\n    print('hello')\n",
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

	// First commit by Nathanael on main (yesterday)
	mainCommit, err := worktree.Commit("First commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Nathanael Farley",
			Email: "nathanael@example.com",
			When:  time.Now().Add(-24 * time.Hour),
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
		"test.txt":    "line1\nline2 modified\nline3",
		"script.py":   "def hello():\n    print('hello world')\n    return True\n",
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
			When:  time.Now(),
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
		"test.txt":    "line1\nline2\nline3 modified",
		"config.yml":  "name: test\nversion: 2.0\nenv: prod\n",
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
			When:  time.Now(),
		},
	})
	assert.NoError(t, err)

	return dir, func() {
		os.RemoveAll(dir)
	}
}

func TestRunLines(t *testing.T) {
	tests := []struct {
		name          string
		authorRegex   string
		filenamesRegex string
		remote        string
		paths         []string
		wantAdded     int64
		wantDeleted   int64
		wantErr       bool
	}{
		{
			name:        "Match Nathanael on main branch",
			authorRegex: "Nathanael",
			paths:       []string{"./@main"},
			wantAdded:   3, // Modified one line in test.txt and added one line in config.yml
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
			name:          "Match Python files only",
			filenamesRegex: "\\.py$",
			paths:         []string{"./@feature"},
			wantAdded:     2, // Only script.py changes in feature branch
			wantDeleted:   1,
		},
		{
			name:          "Match YAML files only",
			filenamesRegex: "\\.ya?ml$",
			paths:         []string{"./@main"},
			wantAdded:     2, // Only config.yml changes in main branch
			wantDeleted:   1,
		},
		{
			name:          "Match Python and YAML files",
			filenamesRegex: "(\\.py$|\\.ya?ml$)",
			paths:         []string{"./@main", "./@feature"},
			wantAdded:     4, // Both script.py and config.yml changes
			wantDeleted:   2,
		},
		{
			name:          "Match Nathanael's YAML changes",
			authorRegex:   "Nathanael",
			filenamesRegex: "\\.ya?ml$",
			paths:         []string{"./@main"},
			wantAdded:     2, // Only config.yml changes by Nathanael
			wantDeleted:   1,
		},
		{
			name:          "Invalid filename regex",
			filenamesRegex: "(",
			paths:         []string{"./@main"},
			wantErr:       true,
		},
		{
			name:        "Match no authors",
			authorRegex: "NonExistent",
			paths:      []string{"./@main", "./@feature"},
			wantAdded:  0,
			wantDeleted: 0,
		},
	}

	dir, cleanup := setupTestRepo(t)
	defer cleanup()

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

			// Change paths to use the temp directory
			paths := make([]string, len(tt.paths))
			for i, p := range tt.paths {
				paths[i] = filepath.Join(dir, p)
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
