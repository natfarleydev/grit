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
	"github.com/go-git/go-git/v5/config"
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

	// Create a test file with 3 lines
	testFile := filepath.Join(dir, "test.txt")
	err = ioutil.WriteFile(testFile, []byte("line1\nline2\nline3"), 0644)
	assert.NoError(t, err)

	// Setup the worktree
	worktree, err := repo.Worktree()
	assert.NoError(t, err)

	// Add and commit with different authors
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

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

	// Modify file in feature branch - change 1 line
	err = ioutil.WriteFile(testFile, []byte("line1\nline2 modified\nline3"), 0644)
	assert.NoError(t, err)

	// Commit by Mirabel in feature branch
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

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

	// Modify file in main branch - change 1 line
	err = ioutil.WriteFile(testFile, []byte("line1\nline2\nline3 modified"), 0644)
	assert.NoError(t, err)

	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

	_, err = worktree.Commit("Main branch commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Nathanael Farley",
			Email: "nathanael@example.com",
			When:  time.Now(),
		},
	})
	assert.NoError(t, err)

	// Set up remote
	_, err = git.PlainInit(filepath.Join(dir, "remote.git"), true)
	assert.NoError(t, err)

	// Add remote
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{filepath.Join(dir, "remote.git")},
	})
	assert.NoError(t, err)

	// Push both branches to remote
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/heads/*:refs/heads/*"),
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
		remote      string
		paths       []string
		wantAdded   int64
		wantDeleted int64
	}{
		{
			name:        "Match Nathanael on main branch",
			authorRegex: "Nathanael",
			paths:       []string{"./@main"},
			wantAdded:   1, // Modified one line in main
			wantDeleted: 1,
		},
		{
			name:        "Match Mirabel on feature branch",
			authorRegex: "Mirabel",
			paths:       []string{"./@feature"},
			wantAdded:   1, // Modified one line in feature
			wantDeleted: 1,
		},
		{
			name:        "Match Nathanael on remote main branch",
			authorRegex: "Nathanael",
			remote:      "origin",
			paths:       []string{"./@main"},
			wantAdded:   1, // Modified one line in main
			wantDeleted: 1,
		},
		{
			name:        "Match Mirabel on remote feature branch",
			authorRegex: "Mirabel",
			remote:      "origin",
			paths:       []string{"./@feature"},
			wantAdded:   1, // Modified one line in feature
			wantDeleted: 1,
		},
		{
			name:        "Match both authors on current branch",
			authorRegex: "(Nathanael|Mirabel)",
			paths:       []string{"./"},
			wantAdded:   1, // Total additions on current branch (main)
			wantDeleted: 1,
		},
		{
			name:        "Match no authors",
			authorRegex: "NonExistent",
			paths:       []string{"./@main", "./@feature"},
			wantAdded:   0,
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

			// Check output format
			expected := fmt.Sprintf("+%d/-%d", tt.wantAdded, tt.wantDeleted)
			assert.Equal(t, expected, buf.String())
		})
	}
}
