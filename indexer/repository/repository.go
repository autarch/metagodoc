package repository

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/autarch/metagodoc/indexer/esmodels"
	"github.com/google/go-github/github"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
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
	clone      *git.Repository
	head       *plumbing.Reference
	ctx        context.Context

	isGoCore bool

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
	isGoCore := id == "github.org/golang/go"
	repo := &Repository{
		Repository: ghr,
		github:     github,
		httpClient: httpClient,
		ctx:        ctx,
		isGoCore:   isGoCore,
		ID:         id,
		VCS:        Git,
	}
	repo.clone, repo.head = repo.getGitRepo(id, ghr.GetCloneURL(), cacheRoot)
	repo.Status = repo.getStatus()
	return repo
}

func (repo *Repository) getGitRepo(id, url, cacheRoot string) (*git.Repository, *plumbing.Reference) {
	path := filepath.Join(cacheRoot, id)
	var c *git.Repository
	if pathExists(path) {
		log.Printf("  %s exists at %s - fetching", id, path)
		var err error
		c, err = git.PlainOpen(path)
		if err != nil {
			log.Panic(err)
		}
		err = c.Fetch(&git.FetchOptions{Tags: git.AllTags})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			log.Panic(err)
		}

	} else {
		log.Printf("  %s does not exist at %s - cloning", id, path)
		var err error
		c, err = git.PlainClone(path, true, &git.CloneOptions{URL: url, Tags: git.AllTags})
		if err != nil {
			log.Panic(err)
		}
	}

	head, err := c.Head()
	if err != nil {
		log.Panic(err)
	}

	return c, head
}

func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	} else if err != nil {
		log.Panic(err)
	}
	return true
}

// A repository with no commits within the last 2 years will be considered
// inactive. But if another active repo imports this one then we will consider
// this one active.
const twoYears = 2 * 365 * 24 * time.Hour

func (repo *Repository) getStatus() ActivityStatus {
	commits, err := repo.clone.Log(&git.LogOptions{})
	if err != nil {
		log.Panic(err)
	}

	firstThree := make([]*object.Commit, 0)
	for c, err := commits.Next(); c != nil; c, err = commits.Next() {
		if err != nil {
			log.Panic(err)
		}
		firstThree = append(firstThree, c)
		if len(firstThree) == 3 {
			break
		}
	}

	if time.Now().Sub(firstThree[0].Author.When) > twoYears {
		return NoRecentCommits
	}

	if repo.GetFork() {
		if repo.GetPushedAt().Before(repo.GetCreatedAt().Time) {
			return DeadEndFork
		} else if repo.isQuickFork(firstThree) {
			return QuickFork
		}
	}

	return Active
}

const oneWeek = 7 * 24 * time.Hour

// isQuickFork reports whether the repository is a "quick fork": it has fewer
// than 3 commits, all within a week of the repo creation, createdAt.  Commits
// must be in reverse chronological order by Commit.Committer.Date.
func (repo *Repository) isQuickFork(firstThree []*object.Commit) bool {
	oneWeekOld := repo.GetCreatedAt().Add(oneWeek)
	if oneWeekOld.After(time.Now()) {
		return false // a newborn baby of a repository
	}
	for _, c := range firstThree {
		if c.Author.When.After(oneWeekOld) {
			return false
		}
		if c.Author.When.Before(repo.GetCreatedAt().Time) {
			break
		}
	}
	return true
}

func (repo *Repository) getIssuesAndPullRequests() (*esmodels.Tickets, *esmodels.Tickets) {
	return &esmodels.Tickets{}, &esmodels.Tickets{}
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
	files := repo.refToFiles(repo.head)
	defer files.Close()

	about := &esmodels.About{}
	err := files.ForEach(func(f *object.File) error {
		m := regexp.MustCompile(`^README(?:\.(md|txt))`).FindStringSubmatch(f.Name)
		if m == nil {
			return nil
		}

		contentType := "text/plain"
		if m[1] == "md" {
			contentType = "text/markdown"
		}

		c, err := f.Contents()
		if err != nil {
			log.Panic(err)
		}
		about = &esmodels.About{Content: c, ContentType: contentType}
		return storer.ErrStop
	})
	if err != nil {
		log.Panic(err)
	}

	return about
}

func (repo *Repository) refToFiles(ref *plumbing.Reference) *object.FileIter {
	o, err := repo.clone.CommitObject(ref.Hash())
	if err != nil {
		log.Panic(err)
	}
	tree, err := o.Tree()
	if err != nil {
		log.Panic(err)
	}
	return tree.Files()
}

func (repo *Repository) getRefs() []*esmodels.Ref {
	refs := []*esmodels.Ref{&esmodels.Ref{
		Name:           repo.head.Name().Short(),
		IsHead:         true,
		LastSeenCommit: repo.head.Hash().String(),
		Packages:       repo.getPackages(repo.head),
	}}

	tags, err := repo.clone.Tags()
	if err != nil {
		log.Panic(err)
	}

	var re *regexp.Regexp
	if repo.isGoCore {
		re = regexp.MustCompile(`^(go[0-9]+\.[0-9]+[^/]*)$`)
	} else {
		re = regexp.MustCompile(`^v?[0-9]+\.[0-9]+[^/]*$`)
	}
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		if !re.MatchString(ref.Name().Short()) {
			// log.Printf("  %s does not match", ref.Name().Short())
			return nil
		}
		// log.Printf("  %s matches", ref.Name().Short())
		r := &esmodels.Ref{
			Name:           ref.Name().Short(),
			IsHead:         false,
			LastSeenCommit: ref.Hash().String(),
			Packages:       repo.getPackages(ref),
		}
		refs = append(refs, r)
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return refs
}

func (repo *Repository) getPackages(ref *plumbing.Reference) []*esmodels.Package {
	log.Printf("    packages for %s", ref.Name().Short())

	iter := repo.refToFiles(ref)

	pmap := make(map[string]*esmodels.Package)
	err := iter.ForEach(func(f *object.File) error {
		if !regexp.MustCompile(`\.go$`).MatchString(f.Name) {
			return nil
		}

		fullName := path.Dir(f.Name)
		if _, e := pmap[fullName]; e {
			return nil
		}

		pmap[fullName] = &esmodels.Package{
			Name:       path.Base(fullName),
			FullName:   fullName,
			ImportPath: path.Join(repo.ID, fullName),
			IsCommand:  false,
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	pkgs := make([]*esmodels.Package, 0)
	for _, p := range pmap {
		pkgs = append(pkgs, p)
	}
	return pkgs
}

// func (repo *Repository) recurseSubdirectories(pkg *doc.Package, depth int) []*esmodels.Package {
// 	indent := strings.Repeat("  ", depth)
// 	log.Printf("%s  looking for subdirectories in %s", indent, pkg.ImportPath)

// 	if len(pkg.Subdirectories) == 0 {
// 		log.Printf("%s  none found", indent)
// 		return nil
// 	}

// 	pkgs := make([]*esmodels.Package, 0)
// 	for _, d := range pkg.Subdirectories {
// 		if d == "internal" {
// 			continue
// 		}
// 		if d == "vendor" {
// 			continue
// 		}

// 		log.Printf("%s  found %s", indent, d)
// 		p, err := doc.Get(repo.ctx, repo.httpClient, filepath.Join(pkg.ImportPath, d), "")
// 		if err != nil {
// 			log.Panic(err)
// 		}

// 		pkgs = append(pkgs, makeEsPackage(p)...)
// 		sub := repo.recurseSubdirectories(p, depth+1)
// 		pkgs = append(pkgs, sub...)
// 	}

// 	return pkgs
// }

var statusMap = map[ActivityStatus]string{
	Active:          "active",
	DeadEndFork:     "dead-end-fork",
	QuickFork:       "quick-fork",
	NoRecentCommits: "no-recent-commits",
	Inactive:        "inactive",
}

func (st ActivityStatus) String() string {
	if v, ok := statusMap[st]; ok {
		return v
	}
	log.Panic("Invalid activity status: %d", st)
	return ""
}

func (repo *Repository) ESModel() *esmodels.Repository {
	issues, prs := repo.getIssuesAndPullRequests()
	return &esmodels.Repository{
		Name:         repo.GetName(),
		FullName:     repo.GetFullName(),
		Description:  repo.GetDescription(),
		PrimaryURL:   repo.GetHTMLURL(),
		Issues:       issues,
		PullRequests: prs,
		Owner:        repo.GetOwner().GetLogin(),
		Created:      repo.GetCreatedAt().UTC().Format(esmodels.DateTimeFormat),
		LastUpdated:  repo.GetPushedAt().Format(esmodels.DateTimeFormat),
		LastCrawled:  time.Now().UTC().Format(esmodels.DateTimeFormat),
		Stars:        repo.GetStargazersCount(),
		Forks:        repo.GetForksCount(),
		Status:       repo.Status.String(),
		About:        repo.getReadme(),
		IsFork:       repo.GetFork(),
		Refs:         repo.getRefs(),
	}
}
