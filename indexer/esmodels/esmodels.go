package esmodels

type Repository struct {
	Name         string   `json:"name"`
	FullName     string   `json:"full_name"`
	Description  string   `json:"description"`
	PrimaryURL   string   `json:"primary_url"`
	Issues       *Tickets `json:"issues"`
	PullRequests *Tickets `json:"pull_requests"`
	Owner        string   `json:"owner"`
	Created      string   `json:"created"`
	LastUpdated  string   `json:"last_updated"`
	LastCrawled  string   `json:"last_crawled"`
	Stars        int      `json:"stars"`
	Forks        int      `json:"forks"`
	IsFork       bool     `json:"is_fork"`
	Status       string   `json:"status"`
	About        *About   `json:"about"`
	Refs         []*Ref   `json:"refs"`
}

type Tickets struct {
	Url    string `json:"url"`
	Open   int    `json:"open"`
	Closed int    `json:"closed"`
}

type About struct {
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
}

type Ref struct {
	Name           string     `json:"name"`
	IsHead         bool       `json:"is_head"`
	LastSeenCommit string     `json:"last_seen_commit"`
	Packages       []*Package `json:"packages"`
}

type Package struct {
	Name         string   `json:"name"`
	ImportPath   string   `json:"import_path"`
	IsCommand    bool     `json:"is_command"`
	Files        []string `json:"files"`
	TestFiles    []string `json:"test_files"`
	XTestFiles   []string `json:"x_test_files"`
	Imports      []string `json:"imports"`
	TestImports  []string `json:"test_imports"`
	XTestImports []string `json:"x_test_imports"`
}

const DateTimeFormat = "2006-01-02T15:04:05"
