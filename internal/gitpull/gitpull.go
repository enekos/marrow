package gitpull

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sync clones or pulls a Git repository and returns the list of changed file paths.
// For private repos, embed the token into the URL: https://<token>@github.com/owner/repo.git
func Sync(repoURL, token, localPath string) ([]string, error) {
	authURL, err := injectToken(repoURL, token)
	if err != nil {
		return nil, fmt.Errorf("invalid repo url: %w", err)
	}

	gitDir := filepath.Join(localPath, ".git")
	_, err = os.Stat(gitDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(localPath, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir: %w", err)
		}
		if err := runGit(localPath, "clone", "--depth", "1", authURL, "."); err != nil {
			return nil, fmt.Errorf("clone: %w", err)
		}
		// First clone: return all .md files under localPath
		return findMarkdownFiles(localPath)
	}
	if err != nil {
		return nil, fmt.Errorf("stat git dir: %w", err)
	}

	// Ensure this is a git repo
	if err := runGit(localPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("not a git repo: %w", err)
	}

	// Fetch latest commit shallowly
	if err := runGit(localPath, "fetch", "--depth", "1", "origin"); err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	// Capture pre-update HEAD for diff
	oldHead, _ := gitOutput(localPath, "rev-parse", "HEAD")

	// Reset to fetched HEAD (works reliably for shallow clones)
	if err := runGit(localPath, "reset", "--hard", "FETCH_HEAD"); err != nil {
		return nil, fmt.Errorf("reset: %w", err)
	}

	newHead, _ := gitOutput(localPath, "rev-parse", "HEAD")

	var changed []string
	if oldHead != "" && oldHead != newHead {
		out, err := gitOutput(localPath, "diff", "--name-only", oldHead, newHead)
		if err != nil {
			return nil, fmt.Errorf("diff: %w", err)
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && strings.HasSuffix(strings.ToLower(line), ".md") {
				changed = append(changed, filepath.Join(localPath, line))
			}
		}
	}
	return changed, nil
}

func injectToken(repoURL, token string) (string, error) {
	if token == "" {
		return repoURL, nil
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(token, "x-oauth-basic")
	return u.String(), nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w (output: %s)", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func findMarkdownFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
