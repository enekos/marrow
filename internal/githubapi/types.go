package githubapi

import "time"

// IssueDocument holds normalized issue data ready for indexing.
type IssueDocument struct {
	Number    int
	Title     string
	Body      string
	URL       string
	UpdatedAt time.Time
	Comments  []string
}

// PullRequestDocument holds normalized pull request data ready for indexing.
type PullRequestDocument struct {
	Number    int
	Title     string
	Body      string
	URL       string
	UpdatedAt time.Time
	Comments  []string
}
