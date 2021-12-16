package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ObjectType represents the type of a Git object ("blob", "tree",
// "commit", "tag", or "missing").
type ObjectType string

// Repository represents a Git repository on disk.
type Repository struct {
	path string

	// gitBin is the path of the `git` executable that should be used
	// when running commands in this repository.
	gitBin string
}

// smartJoin returns the path that can be described as `relPath`
// relative to `path`, given that `path` is either absolute or is
// relative to the current directory.
func smartJoin(path, relPath string) string {
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(path, relPath)
}

// GitDir gets repo's git-dir
func GitDir(gitbin, path string) (string, error) {
	cmd := exec.Command(gitbin, "-C", path, "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		switch err := err.(type) {
		case *exec.Error:
			return "", fmt.Errorf(
				"could not run '%s': %w", gitbin, err.Err,
			)
		case *exec.ExitError:
			return "", fmt.Errorf(
				"git rev-parse failed: %s", err.Stderr,
			)
		default:
			return "", err
		}
	}
	gitDir := smartJoin(path, string(bytes.TrimSpace(out)))

	return gitDir, nil
}

// IsShallow checks if a repo is shallow clone
func IsShallow(gitbin, gitdir string) (bool, error) {
	cmd := exec.Command(gitbin, "rev-parse", "--git-path", "shallow")
	cmd.Dir = gitdir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf(
			"could not run 'git rev-parse --git-path shallow': %w", err,
		)
	}
	shallow := smartJoin(gitdir, string(bytes.TrimSpace(out)))
	_, err = os.Lstat(shallow)
	if err == nil {
		return true, errors.New("this appears to be a shallow clone; full clone required")
	}
	return false, nil
}

// NewRepository creates a new repository object that can be used for
// running `git` commands within that repository.
func NewRepository(path string) (*Repository, error) {
	// Find the `git` executable to be used:
	gitBin, err := findGitBin()
	if err != nil {
		return nil, fmt.Errorf(
			"could not find 'git' executable (is it in your PATH?): %w", err,
		)
	}
	// Find git dir
	gitDir, err := GitDir(gitBin, path)
	if err != nil {
		return nil, err
	}
	// Check if the repo is a shallow clone
	shallow, err := IsShallow(gitBin, gitDir)
	if shallow {
		return nil, err
	}
	return &Repository{
		path:   gitDir,
		gitBin: gitBin,
	}, nil
}

func (repo *Repository) GitCommand(callerArgs ...string) *exec.Cmd {
	args := []string{
		// Disable replace references when running our commands:
		"--no-replace-objects",

		// Disable the warning that grafts are deprecated, since we
		// want to set the grafts file to `/dev/null` below (to
		// disable grafts even where they are supported):
		"-c", "advice.graftFileDeprecated=false",
	}

	args = append(args, callerArgs...)

	//nolint:gosec // `gitBin` is chosen carefully, and the rest of
	// the args have been checked.
	cmd := exec.Command(repo.gitBin, args...)

	cmd.Env = append(
		os.Environ(),
		"GIT_DIR="+repo.path,
		// Disable grafts when running our commands:
		"GIT_GRAFT_FILE="+os.DevNull,
	)

	return cmd
}

// Path returns the path to `repo`.
func (repo *Repository) Path() string {
	return repo.path
}
