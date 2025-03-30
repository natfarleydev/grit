package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func setupIntegrationTestRepo(t *testing.T) (string, func()) {
	// Create a temporary directory
	dir, err := os.MkdirTemp("", "grit-integration-test")
	assert.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(dir, false)
	assert.NoError(t, err)

	// Create test files with different authors
	files := []struct {
		name       string
		content    string
		newContent string
		author     string
		email      string
	}{
		{
			name:       "file1.py",
			content:    "Initial content\nfor file 1\n",
			newContent: "Updated content\nfor file 1\nby Nathanael\n",
			author:     "Nathanael Farley",
			email:      "nathanael@example.com",
		},
		{
			name:       "file2.txt",
			content:    "Initial content\nfor file 2\n",
			newContent: "Updated content\nfor file 2\nby Mirabel\n",
			author:     "Mirabel Smith",
			email:      "mirabel@example.com",
		},
	}

	w, err := repo.Worktree()
	assert.NoError(t, err)

	// First create and commit initial content
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		err = os.WriteFile(path, []byte(f.content), 0644)
		assert.NoError(t, err)

		_, err = w.Add(f.name)
		assert.NoError(t, err)

		_, err = w.Commit("Initial "+f.name, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Initial Author",
				Email: "initial@example.com",
				When:  time.Now().Add(-24 * time.Hour), // Yesterday
			},
		})
		assert.NoError(t, err)
	}

	// Then update and commit changes with different authors
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		err = os.WriteFile(path, []byte(f.newContent), 0644)
		assert.NoError(t, err)

		_, err = w.Add(f.name)
		assert.NoError(t, err)

		_, err = w.Commit("Update "+f.name, &git.CommitOptions{
			Author: &object.Signature{
				Name:  f.author,
				Email: f.email,
				When:  time.Now(),
			},
		})
		assert.NoError(t, err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

func TestCommandLineInterface(t *testing.T) {
	dir, cleanup := setupIntegrationTestRepo(t)
	defer cleanup()

	// Get project root directory
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filename)

	// Build the binary
	binaryName := "gritforintegrationtest"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(projectRoot, binaryName)

	// Clean up old binary if it exists
	os.Remove(binaryPath)

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	buildCmd.Dir = projectRoot
	buildCmd.Env = append(os.Environ(), "GO111MODULE=on")
	err := buildCmd.Run()
	assert.NoError(t, err)
	defer os.Remove(binaryPath)

	tests := []struct {
		name    string
		args    []string
		wantOut string
		wantErr bool
	}{
		{
			name:    "count lines for everyone",
			args:    []string{"count", "lines", "."},
			wantOut: "+4/-2", // Updated content: 2 new lines, 1 deleted line
			wantErr: false,
		},
		{
			name:    "count lines for (Nathanael|Mirabel|Louisa)",
			args:    []string{"count", "lines", "--author-regex", "(Nathanael|Mirabel|Louisa)", "."},
			wantOut: "+4/-2", // Updated content: 2 new lines, 1 deleted line
			wantErr: false,
		},
		{
			name:    "count lines for Nathanael",
			args:    []string{"count", "lines", "--author-regex", "Nathanael", "."},
			wantOut: "+2/-1", // Updated content: 2 new lines, 1 deleted line
			wantErr: false,
		},
		{
			name:    "count lines for Mirabel",
			args:    []string{"count", "lines", "--author-regex", "Mirabel", "."},
			wantOut: "+2/-1", // Updated content: 2 new lines, 1 deleted line
			wantErr: false,
		},
		{
			name:    "count lines for (py$|yml$)",
			args:    []string{"count", "lines", "--filenames-regex", "(py$|yml$)", "."},
			wantOut: "+2/-1",
			wantErr: false,
		},
		{
			name:    "count lines for (py$|txt$)",
			args:    []string{"count", "lines", "--filenames-regex", "(py$|txt$)", "."},
			wantOut: "+4/-2",
			wantErr: false,
		},
		{
			name:    "log command",
			args:    []string{"log"},
			wantErr: false,
		},
		{
			name:    "invalid command",
			args:    []string{"invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wantOut != "" {
					assert.Contains(t, strings.TrimSpace(string(output)), tt.wantOut)
				}
			}
		})
	}
}
