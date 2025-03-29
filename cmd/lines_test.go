package cmd

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
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

	// Create a test file
	testFile := filepath.Join(dir, "test.txt")
	err = ioutil.WriteFile(testFile, []byte("initial content"), 0644)
	assert.NoError(t, err)

	// Setup the worktree
	worktree, err := repo.Worktree()
	assert.NoError(t, err)

	// Add and commit with different authors
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

	// First commit by Nathanael
	_, err = worktree.Commit("First commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Nathanael Farley",
			Email: "nathanael@example.com",
			When:  time.Now(),
		},
	})
	assert.NoError(t, err)

	// Modify file
	err = ioutil.WriteFile(testFile, []byte("modified content\nby Mirabel"), 0644)
	assert.NoError(t, err)

	// Second commit by Mirabel
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

	_, err = worktree.Commit("Second commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Mirabel Smith",
			Email: "mirabel@example.com",
			When:  time.Now(),
		},
	})
	assert.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

func TestRunLines(t *testing.T) {
	tests := []struct {
		name        string
		authorRegex string
		wantAdded   int64
		wantDeleted int64
	}{
		{
			name:        "Match Nathanael",
			authorRegex: "Nathanael",
			wantAdded:   15, // Length of "initial content"
			wantDeleted: 0,
		},
		{
			name:        "Match Mirabel",
			authorRegex: "Mirabel",
			wantAdded:   25, // Length of "modified content\nby Mirabel"
			wantDeleted: 15, // Length of "initial content"
		},
		{
			name:        "Match both authors",
			authorRegex: "(Nathanael|Mirabel)",
			wantAdded:   40, // Total additions
			wantDeleted: 15, // Total deletions
		},
		{
			name:        "Match no authors",
			authorRegex: "NonExistent",
			wantAdded:   0,
			wantDeleted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, cleanup := setupTestRepo(t)
			defer cleanup()

			// Capture output
			r, pipeW, _ := os.Pipe()
			oldStdout := os.Stdout
			os.Stdout = pipeW
			defer func() {
				pipeW.Close()
				os.Stdout = oldStdout
			}()

			// Set up and run command
			authorRegex = tt.authorRegex
			runLines(nil, []string{dir})

			// Get output
			pipeW.Close()
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Verify output format
			assert.Regexp(t, `\+\d+/-\d+`, output)
		})
	}
}
