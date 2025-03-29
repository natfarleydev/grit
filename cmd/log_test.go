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

func TestRunLog(t *testing.T) {
	// Create a temporary directory
	dir, err := ioutil.TempDir("", "grit-log-test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	// Initialize git repo
	repo, err := git.PlainInit(dir, false)
	assert.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(dir, "test.txt")
	err = ioutil.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Setup the worktree
	worktree, err := repo.Worktree()
	assert.NoError(t, err)

	// Add and commit
	_, err = worktree.Add("test.txt")
	assert.NoError(t, err)

	commitTime := time.Now()
	commitHash, err := worktree.Commit("Test commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test Author",
			Email: "test@example.com",
			When:  commitTime,
		},
	})
	assert.NoError(t, err)

	// Capture output
	r, pipeW, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = pipeW
	defer func() {
		pipeW.Close()
		os.Stdout = oldStdout
	}()

	// Run log command
	runLog(nil, []string{dir})

	// Get output
	pipeW.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected information
	assert.Contains(t, output, commitHash.String()[:7])
	assert.Contains(t, output, "Test Author")
	assert.Contains(t, output, "test@example.com")
	assert.Contains(t, output, "Test commit")
}
