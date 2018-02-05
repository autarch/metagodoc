package esmodels

type Repository struct {
	Name         string     `json:"name"`
	FullName     string     `json:"full_name"`
	Description  string     `json:"description"`
	PrimaryURL   string     `json:"primary_url"`
	Issues       *Tickets   `json:"issues"`
	PullRequests *Tickets   `json:"pull_requests"`
	Owner        string     `json:"owner"`
	Created      string     `json:"created"`
	LastUpdated  string     `json:"last_updated"`
	LastCarwled  string     `json:"last_crawled"`
	Stars        int        `json:"stars"`
	Forks        int        `json:"forks"`
	IsFork       bool       `json:"is_fork"`
	Status       string     `json:"status"`
	About        *About     `json:"about"`
	Packages     []*Package `json:"packages"`
	Etag         string     `json:"etag"`
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

type Package struct {
	Name       string `json:"name"`
	ImportPath string `json:"import_path"`
	IsCommand  bool   `json:"is_command"`
}
