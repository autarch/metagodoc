package repository

import (
	"github.com/autarch/metagodoc/indexer/doc"
)

type VCSType string

const (
	Git VCSType = "Git"
	Hg          = "Hg"
	SVN         = "SVN"
	Bzr         = "Bzr"
)

type Repository interface {
	ESModel() *ESRepository
	ID() string
}

type ESRepository struct {
	Name         string   `json:"name" esType:"keyword"`
	FullName     string   `json:"full_name" esType:"keyword"`
	Description  string   `json:"description" esType:"text" esAnalyzer:"english"`
	VCS          string   `json:"vcs" esType:"keyword"`
	PrimaryURL   string   `json:"primary_url" esType:"keyword"`
	Issues       *Tickets `json:"issues"`
	PullRequests *Tickets `json:"pull_requests"`
	Owner        string   `json:"owner" esType:"keyword"`
	Created      string   `json:"created" esType:"date"`
	LastUpdated  string   `json:"last_updated" esType:"date"`
	LastCrawled  string   `json:"last_crawled" esType:"date"`
	Stars        int      `json:"stars" esType:"long"`
	Forks        int      `json:"forks" esType:"long"`
	IsFork       bool     `json:"is_fork" esType:"boolean"`
	Status       string   `json:"status" esType:"keyword"`
	About        *About   `json:"about""`
	Refs         []*Ref   `json:"refs"`
}

type Tickets struct {
	URL    string `json:"url" esType:"keyword"`
	Open   int    `json:"open" esType:"long"`
	Closed int    `json:"closed" esType:"long"`
}

type About struct {
	Content     string `json:"content" esType:"text" esAnalyzer:"english"`
	ContentType string `json:"content_type" esType:"keyword"`
}

type Package struct {
	Name         string                 `json:"name" esType:"keyword"`
	ImportPath   string                 `json:"import_path" esType:"keyword"`
	Doc          string                 `json:"doc" esType:"text" esAnalyzer:"english"`
	Synopsis     string                 `json:"synopsis" esType:"text" esAnalyzer:"english"`
	Errors       []string               `json:"errors" esType:"keyword"`
	IsCommand    bool                   `json:"is_command" esType:"boolean"`
	Files        []*doc.File            `json:"files"`
	TestFiles    []*doc.File            `json:"test_files"`
	Imports      []string               `json:"imports" esType:"keyword"`
	TestImports  []string               `json:"test_imports" esType:"keyword"`
	XTestImports []string               `json:"x_test_imports" esType:"keyword"`
	Consts       []*doc.Value           `json:"consts"`
	Funcs        []*doc.Func            `json:"funcs"`
	Types        []*doc.Type            `json:"types"`
	Vars         []*doc.Value           `json:"vars"`
	Examples     []*doc.Example         `json:"examples"`
	Notes        map[string][]*doc.Note `json:"notes"`
}

type Ref struct {
	Name            string     `json:"name" esType:"keyword"`
	IsDefaultBranch bool       `json:"is_head" esType:"boolean"`
	RefType         string     `json:"ref_type" esType:"keyword"`
	LastSeenCommit  string     `json:"last_seen_commit" esType:"keyword"`
	LastUpdated     string     `json:"last_updated" esType:"date"`
	Packages        []*Package `json:"packages"`
}
