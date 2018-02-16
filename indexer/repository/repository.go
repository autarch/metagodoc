package repository

import (
	"context"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/autarch/metagodoc/indexer/esmodels"

	"github.com/google/go-github/github"
	version "github.com/hashicorp/go-version"
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
	isGoCore   bool
	cloneRoot  string

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

var skipList map[string]bool = map[string]bool{
	// A slide deck?
	"github.com/GoesToEleven/GolangTraining": true,
	"github.com/golang/go":                   true,
	// Contains invalid .go file (no package).
	"github.com/qiniu/gobook": true,
	// // A book.
	"github.com/adonovan/gopl.io": true,
	"github.com/aws/aws-sdk-go":   true,
}

func New(ghr *github.Repository, github *github.Client, httpClient *http.Client, cacheRoot string, ctx context.Context) *Repository {
	id := regexp.MustCompile(`^https?://`).ReplaceAllString(ghr.GetHTMLURL(), "")
	log.Printf("Indexing %s", id)

	if skipList[id] {
		log.Print("  is on the skip list")
		return nil
	}

	isGoCore := id == "github.com/golang/go"
	repo := &Repository{
		Repository: ghr,
		github:     github,
		httpClient: httpClient,
		ctx:        ctx,
		isGoCore:   isGoCore,
		cloneRoot:  filepath.Join(cacheRoot, "repos", id),
		ID:         id,
		VCS:        Git,
	}
	repo.clone, repo.head = repo.getGitRepo()
	repo.Status = repo.getStatus()
	return repo
}

func (repo *Repository) getGitRepo() (*git.Repository, *plumbing.Reference) {
	var c *git.Repository
	if pathExists(repo.cloneRoot) {
		log.Printf("  %s exists at %s - fetching", repo.ID, repo.cloneRoot)
		var err error
		c, err = git.PlainOpen(repo.cloneRoot)
		if err != nil {
			log.Panic(err)
		}
		err = c.Fetch(&git.FetchOptions{Tags: git.AllTags})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			log.Panic(err)
		}
		// We need to make sure that we're at the HEAD for later operations.
		checkOutRef(c, nil)
	} else {
		log.Printf("  %s does not exist at %s - cloning", repo.ID, repo.cloneRoot)
		var err error
		c, err = git.PlainClone(repo.cloneRoot, false, &git.CloneOptions{URL: repo.GetCloneURL(), Tags: git.AllTags})
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

func checkOutRef(c *git.Repository, ref *plumbing.Reference) {
	wt, err := c.Worktree()
	if err != nil {
		log.Panic(err)
	}
	err = wt.Clean(&git.CleanOptions{Dir: true})
	if err != nil {
		log.Panic(err)
	}
	co := &git.CheckoutOptions{Force: true}
	if ref != nil {
		log.Printf("Checkout %s", ref.Hash().String())
		co.Hash = ref.Hash()
	} else {
		log.Print("Checkout HEAD")
	}
	err = wt.Checkout(co)
	if err != nil {
		log.Panic(err)
	}
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

	tagIter, err := repo.clone.Tags()
	if err != nil {
		log.Panic(err)
	}

	var re *regexp.Regexp
	if repo.isGoCore {
		re = regexp.MustCompile(`^go[0-9]+(?:\.[0-9]+)*$`)
	} else {
		re = regexp.MustCompile(`^v?[0-9]+(?:\.[0-9]+)*$`)
	}

	// We want to go through the refs in sorted order. This should reduce
	// churn in the worktree as checking out versions that are close to each
	// other should require fewer changes to the files. This should speed up
	// the overall indexing process.
	var versions version.Collection
	tags := make(map[*version.Version]*plumbing.Reference)

	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		log.Printf("TAG %s (%s) of type %s", ref.Name().String(), ref.Hash().String(), ref.Type().String())
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		if !re.MatchString(ref.Name().Short()) {
			// log.Printf("  %s does not match", ref.Name().Short())
			return nil
		}
		if ref == repo.head {
			return nil
		}

		name := ref.Name().Short()
		if repo.isGoCore {
			// The version package doesn't like the go core repo's tag names
			// like "go1.0.1".
			name = strings.Replace(name, "go", "", 1)
		}
		v := version.Must(version.NewVersion(name))
		versions = append(versions, v)
		tags[v] = ref

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	sort.Sort(versions)
	i := 0
	for _, v := range versions {
		// XXX - temporarily only index 3 tags
		if i >= 3 {
			break
		}
		i++
		ref := tags[v]
		// log.Printf("  %s matches", ref.Name().Short())
		refs = append(refs, &esmodels.Ref{
			Name:           ref.Name().Short(),
			IsHead:         false,
			LastSeenCommit: ref.Hash().String(),
			Packages:       repo.getPackages(ref),
		})
	}

	return refs
}

func (repo *Repository) getPackages(ref *plumbing.Reference) []*esmodels.Package {
	log.Printf("    packages for %s", ref.Name().Short())
	checkOutRef(repo.clone, ref)
	return repo.walkTreeForPackages(repo.cloneRoot)
}

func (repo *Repository) walkTreeForPackages(dir string) []*esmodels.Package {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Panic(err)
	}

	var p *esmodels.Package = nil
	var pkgs []*esmodels.Package

	for _, f := range files {
		name := f.Name()
		path := filepath.Join(dir, name)
		if f.IsDir() {
			// There are no packages to index outside of the src/ part of go
			// core repo.
			if repo.isGoCore && strings.Index(path, "/src") == -1 {
				continue
			}
			// The core has testdata directories containing go code that
			// should be ignored.
			if repo.isGoCore && name == "testdata" {
				continue
			}
			if name == "." || name == "internal" || name == "vendor" || name == ".git" {
				continue
			}
			pkgs = append(pkgs, repo.walkTreeForPackages(path)...)
		}

		// If we've already seen a .go file in this directory then we've made
		// the package for the directory.
		if p != nil {
			continue
		}

		if regexp.MustCompile(`\.go$`).MatchString(name) {
			p = repo.packageForDir(dir)
		}
	}

	if p != nil {
		return append(pkgs, p)
	}
	return pkgs
}

// There are paths that contain go code in the golang/go repo that are not
// organized in valid manner, for example
// https://github.com/golang/go/tree/master/doc/progs, which contains a bunch
// of example programs, each with its own package.
func (repo *Repository) isGoCorePackage(path string) bool {
	importPath := strings.Replace(path, repo.cloneRoot+"/src", "", 1)
	return pathFlags[importPath]&packagePath != 0
}

func (repo *Repository) packageForDir(dir string) *esmodels.Package {
	bpkg, err := build.ImportDir(dir, build.ImportComment)
	if err != nil {
		// This can happen if the directory contains go code that for some
		// reason cannot be built. For example, the src/cmd/vet/all/main.go
		// file in the golang core repo has a "+build ignore" comment in it
		// that causes it to be ignored, and it's the only go file in that
		// directory.
		if _, ok := err.(*build.NoGoError); ok {
			return nil
		}
		log.Panic(err)
	}

	// For some reason bpkg.ImportPath is always giving me ".". But what I'm
	// doing here is really gross. There's got to be a proper way to get this
	// working.
	var importPath string
	if repo.isGoCore {
		importPath = regexp.MustCompile(`^.+?/src/pkg/`).ReplaceAllLiteralString(dir, "")
	} else {
		importPath = regexp.MustCompile(`^.+?/`+repo.ID).ReplaceAllLiteralString(dir, repo.ID)
	}

	return &esmodels.Package{
		Name:         bpkg.Name,
		ImportPath:   importPath,
		IsCommand:    bpkg.IsCommand(),
		Files:        bpkg.GoFiles,
		TestFiles:    bpkg.TestGoFiles,
		XTestFiles:   bpkg.XTestGoFiles,
		Imports:      bpkg.Imports,
		TestImports:  bpkg.TestImports,
		XTestImports: bpkg.XTestImports,
	}
}

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
