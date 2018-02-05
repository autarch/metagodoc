package repository

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/autarch/gopal/indexer/esmodels"
	"github.com/golang/gddo/doc"
	"github.com/google/go-github/github"
)

type ActivityStatus int

const (
	Active          ActivityStatus = iota
	DeadEndFork                    // Forks with no commits
	QuickFork                      // Forks with less than 3 commits, all within a week from creation
	NoRecentCommits                // No commits for ExpiresAfter

	// No commits for ExpiresAfter and no imports.
	// This is a status derived from NoRecentCommits and the imports count information in the db.
	Inactive
)

type VCSType int

const (
	Git VCSType = iota
	Hg
	SVN
	Bzr
)

type Repository struct {
	*github.Repository
	github     *github.Client
	httpClient *http.Client
	dir        *directory.Directory
	pkg        *doc.Package
	repoRoot   string
	ctx        context.Context

	// A unique ID for the repository based on its URL without the scheme. So
	// for a GitHub repo like "https://github.com/stretchr/testify" this would
	// be "github.com/stretchr/testify". This may be turned into import paths
	// for individual packages.
	ID string

	// Version control system: git, hg, bzr, ...
	VCS VCSType

	// Version control status. Anything but Active will be ignored in search
	// results by default.
	Status ActivityStatus
}

func New(ghr *github.Repository, github *github.Client, httpClient *http.Client, cacheRoot string, ctx context.Context) *Repository {
	id := regexp.MustCompile(`^https?://`).ReplaceAllString(ghr.GetHTMLURL(), "")
	repo := &Repository{
		Repository: ghr,
		github:     github,
		httpClient: httpClient,
		repoRoot:   filepath.Join(cacheRoot, id),
		ctx:        ctx,
		ID:         id,
		VCS:        Git,
	}
	repo.setStatus()
	repo.setGDDO()
	return repo
}

 // A repository with no commits within the last 2 years will be considered
 // inactive. Note that if active repos still import this one's then we may
 // still consider it active.
const twoYears = 2 * 365 * 24 * time.Hour

func (repo *Repository) setStatus() {
	status := Active

	commits, err := repo.github.Repositories.ListCommits(
		repo.ctx,
		repo.GetOwner(),
		repo.GetName(),
		&github.CommitsListOptions{PerPage: 3},
	)
	if err != nil {
		log.Panic(err)
	}
	if len(commits) == 0 {
		log.Panic("Could not find any commits for repo")
	}

	lastCommitted := commits[0].GetCommitter().GetDate()
	if time.Now().Sub(lastCommitted) > twoYears {
		status = NoRecentCommits
	} else if repo.GetFork() {
		if repo.GetPushedAt().Before(repo.GetCreatedAt()) {
			status = DeadEndFork
		} else if repo.isQuickFork(commits) {
			status = QuickFork
		}
	}
}

const oneWeek = 7 * 24 * time.Hour

// isQuickFork reports whether the repository is a "quick fork": it has fewer
// than 3 commits, all within a week of the repo creation, createdAt.  Commits
// must be in reverse chronological order by Commit.Committer.Date.
func (repo *Repository) isQuickFork(commits []*githubCommit) bool {
	oneWeekOld := repo.GetCreatedAt().Add(onWeek)
	if oneWeekOld.After(time.Now()) {
		return false // a newborn baby of a repository
	}
	for _, commit := range commits {
		if commit.GetCommitter().GetDate().After(oneWeekOld) {
			return false
		}
		if commit.Commit.Committer.Date.Before(repo.GetCreatedAt()) {
			break
		}
	}
	return true
}

func (repo *Repository) setGDDO() {
	dir, err := directory.Get(repo.ctx, repo.httpClient, repo.ID, "")
	if err != nil {
		log.Panic(err)
	}

	pkg, err := doc.Get(repo.ctx, repo.httpClient, repo.ID, dir.Etag)
	if err != nil {
		log.Panic(err)
	}

	repo.dir = dir
	repo.pkg = pkg
}

func (repo *Repository) getIssuesAndPullRequests() (*esmodels.Tickets, *esmodels.Tickets) {
	log.Print("  getting issues")

	issues := &esmodels.Tickets{
		Url: fmt.Sprintf("%s/issues", repo.GetHTMLURL()),
	}
	prs := &esmodels.Tickets{
		Url: fmt.Sprintf("%s/pulls", repo.GetHTMLURL()),
	}

	opts := &github.IssueListByRepoOptions{}
	for {
		issuesList, resp, err := repo.github.Issues.ListByRepo(
			repo.ctx,
			repo.GetOwner().GetLogin(),
			repo.GetName(),
			opts,
		)
		if err != nil {
			log.Panic(err)
		}

		for _, i := range issuesList {
			var s *esmodels.Tickets
			if i.IsPullRequest() {
				s = prs
			} else {
				s = issues
			}
			if i.GetClosedAt != nil {
				s.Closed++
			} else {
				s.Open++
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return issues, prs
}

func (repo *Repository) getReadme() *esmodels.About {

	for _, f := range repo.dir.Files {
		m := regexp.MustCompile(`^README(?:\.(md|txt))`).FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}

		contentType := "text/plain"
		if m[1] == "md" {
			contentType = "text/markdown"
		}

		return &esmodels.About{Content: string(f.Data), ContentType: contentType}
	}

	return &esmodels.About{}
}

func (repo *Repository) getPackages() []*esmodels.Package {
	pkgs := append(make([]*esmodels.Package, 0), makeEsPackage(repo.pkg)...)
	sub := repo.recurseSubdirectories(repo.pkg, 0)
	return append(pkgs, sub...)
}

func (repo *Repository) recurseSubdirectories(pkg *doc.Package, depth int) []*esmodels.Package {
	indent := strings.Repeat("  ", depth)
	log.Printf("%s  looking for subdirectories in %s", indent, pkg.ImportPath)

	if len(pkg.Subdirectories) == 0 {
		log.Printf("%s  none found", indent)
		return nil
	}

	pkgs := make([]*esmodels.Package, 0)
	for _, d := range pkg.Subdirectories {
		if d == "internal" {
			continue
		}
		if d == "vendor" {
			continue
		}

		log.Printf("%s  found %s", indent, d)
		p, err := doc.Get(repo.ctx, repo.httpClient, filepath.Join(pkg.ImportPath, d), "")
		if err != nil {
			log.Panic(err)
		}

		pkgs = append(pkgs, makeEsPackage(p)...)
		sub := repo.recurseSubdirectories(p, depth+1)
		pkgs = append(pkgs, sub...)
	}

	return pkgs
}

func makeEsPackage(pkg *doc.Package) []*esmodels.Package {
	// A "package" without any go files will not have a name. A good example
	// is the models in aws-sdk-go, which is a huge directory tree containing
	// JSON files. See https://github.com/aws/aws-sdk-go/tree/master/models.
	if pkg.Name == "" {
		return nil
	}

	return []*esmodels.Package{&esmodels.Package{
		Name:       pkg.Name,
		ImportPath: pkg.ImportPath,
		IsCommand:  pkg.IsCmd,
		Etag:       pkg.Etag,
	}}
}

const statusMap = map[ActivityStatus]string{
	directory.Active:          "active",
	directory.DeadEndFork:     "dead-end-fork",
	directory.QuickFork:       "quick-fork",
	directory.NoRecentCommits: "no-recent-commits",
	directory.Inactive:        "inactive",
}

func (repo *Repository) StatusString() string {
	if v, ok := statusMap[r.Status]; ok {
		return v
	}
	log.Panic("Invalid directory status: %d", s)
}

func (repo *Repository) ESModel() *esmodels.Repository {
	issues, prs := repo.getIssuesAndPullRequests()
	f := "2006-01-02 15:04:05"
	return &esmodels.Repository{
		Name:         repo.GetName(),
		FullName:     repo.GetFullName(),
		Description:  repo.GetDescription(),
		PrimaryURL:   repo.GetHTMLURL(),
		Issues:       issues,
		PullRequests: prs,
		Owner:        repo.GetOwner().GetLogin(),
		Created:      repo.GetCreatedAt().UTC().Format(f),
		LastUpdated:  repo.GetPushedAt().Format(f),
		LastCrawled:  time.Now().UTC().Format(f),
		Stars:        repo.pkg.Stars,
		Forks:        repo.GetForksCount(),
		Status:       repo.StatusString(),
		About:        repo.dir.Readme(),
		IsFork:       repo.GetFork(),
		Packages:     repo.getPackages(),
	}
}
