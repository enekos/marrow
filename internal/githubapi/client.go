package githubapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v69/github"
)

// Client wraps the GitHub API client with App authentication.
type Client struct {
	appID          int64
	installationID int64
	privateKey     []byte
	httpClient     *http.Client
	gh             *github.Client
}

// NewClient creates a GitHub App client. If installationID is 0, it will be
// auto-discovered on the first API call that needs it.
func NewClient(appID int64, privateKey []byte, installationID int64) (*Client, error) {
	if appID == 0 {
		return nil, fmt.Errorf("appID must be > 0")
	}
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("privateKey is required")
	}
	c := &Client{
		appID:          appID,
		privateKey:     privateKey,
		installationID: installationID,
	}
	// If installationID is provided upfront, create the authenticated client now.
	if installationID > 0 {
		if err := c.refreshClient(installationID); err != nil {
			return nil, fmt.Errorf("refresh client: %w", err)
		}
	}
	return c, nil
}

func (c *Client) refreshClient(installationID int64) error {
	itr, err := ghinstallation.New(http.DefaultTransport, c.appID, installationID, c.privateKey)
	if err != nil {
		return fmt.Errorf("ghinstallation: %w", err)
	}
	c.httpClient = &http.Client{Transport: itr}
	c.gh = github.NewClient(c.httpClient)
	c.installationID = installationID
	return nil
}

// setClient is used by tests to inject a mock go-github client.
func (c *Client) setClient(gh *github.Client) {
	c.gh = gh
	c.installationID = 1
}

func (c *Client) ensureInstallation(ctx context.Context, owner, repo string) error {
	if c.installationID > 0 && c.gh != nil {
		return nil
	}
	// Auto-discover installation ID by listing installations.
	// We need an unauthenticated-by-installation client first: use JWT transport.
	jwtTransport, err := ghinstallation.NewAppsTransport(http.DefaultTransport, c.appID, c.privateKey)
	if err != nil {
		return fmt.Errorf("jwt transport: %w", err)
	}
	jwtClient := github.NewClient(&http.Client{Transport: jwtTransport})

	installations, resp, err := jwtClient.Apps.ListInstallations(ctx, &github.ListOptions{PerPage: 100})
	if err != nil {
		return fmt.Errorf("list installations: %w (status %d)", err, resp.StatusCode)
	}

	for _, inst := range installations {
		if inst.GetAccount().GetLogin() == owner {
			return c.refreshClient(inst.GetID())
		}
	}
	return fmt.Errorf("no installation found for owner %q", owner)
}

// parseRepo extracts owner and repo name from a GitHub repo URL.
func parseRepo(repoURL string) (owner, repo string, err error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 3 {
		return parts[1], parts[2], nil
	}
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "", "", fmt.Errorf("invalid repo URL: %s", repoURL)
}
