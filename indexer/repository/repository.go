package repository

import (
	"container/list"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/autarch/metagodoc/esmodels"
	"github.com/autarch/metagodoc/indexer/directory"
	"github.com/autarch/metagodoc/indexer/doc"
	"github.com/golang/gddo/gosrc"

	"code.gitea.io/git"
	"github.com/google/go-github/github"
	version "github.com/hashicorp/go-version"
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

type VCSType string

const (
	Git VCSType = "Git"
	Hg          = "Hg"
	SVN         = "SVN"
	Bzr         = "Bzr"
)

type Repository struct {
	*github.Repository
	github     *github.Client
	httpClient *http.Client
	clone      *git.Repository
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
	repo.clone = repo.getGitRepo()
	return repo
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

func (repo *Repository) ESModel() *ESRepository {
	issues, prs := repo.getIssuesAndPullRequests()
	return &ESRepository{
		Name:         repo.GetName(),
		FullName:     repo.GetFullName(),
		VCS:          string(repo.VCS),
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
		Status:       repo.getStatus().String(),
		About:        repo.getReadme(),
		IsFork:       repo.GetFork(),
		Refs:         repo.getRefs(),
	}
}

func (repo *Repository) getGitRepo() *git.Repository {
	var c *git.Repository

	exists := pathExists(repo.cloneRoot)
	if !exists {
		log.Printf("  %s does not exist at %s - cloning", repo.ID, repo.cloneRoot)
		err := git.Clone(repo.GetCloneURL(), repo.cloneRoot, git.CloneRepoOptions{})
		if err != nil {
			log.Panic(err)
		}
	}

	var err error
	c, err = git.OpenRepository(repo.cloneRoot)
	if err != nil {
		log.Panic(err)
	}

	if exists {
		log.Printf("  %s exists at %s - fetching", repo.ID, repo.cloneRoot)
		_, err = git.NewCommand("fetch", "--tags").RunInDir(c.Path)
		if err != nil {
			log.Panic(err)
		}
	}

	return c
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
	head, err := repo.clone.GetBranchCommit(repo.GetDefaultBranch())
	if err != nil {
		log.Panic(err)
	}

	if time.Now().Sub(head.Author.When) > twoYears {
		return NoRecentCommits
	}

	commits, err := head.CommitsBeforeLimit(2)
	if err != nil {
		log.Panic(err)
	}
	commits.PushFront(head)

	if repo.GetFork() {
		if repo.GetPushedAt().Before(repo.GetCreatedAt().Time) {
			return DeadEndFork
		} else if repo.isQuickFork(commits) {
			return QuickFork
		}
	}

	return Active
}

const oneWeek = 7 * 24 * time.Hour

// isQuickFork reports whether the repository is a "quick fork": it has fewer
// than 3 commits, all within a week of the repo creation, createdAt.  Commits
// must be in reverse chronological order by Commit.Committer.Date.
func (repo *Repository) isQuickFork(firstThree *list.List) bool {
	oneWeekOld := repo.GetCreatedAt().Add(oneWeek)
	if oneWeekOld.After(time.Now()) {
		return false // a newborn baby of a repository
	}
	for e := firstThree.Front(); e != nil; e = e.Next() {
		c := e.Value.(*git.Commit)
		if c.Author.When.After(oneWeekOld) {
			return false
		}
		if c.Author.When.Before(repo.GetCreatedAt().Time) {
			break
		}
	}
	return true
}

type Tickets struct {
	Url    string `json:"url" esType:"keyword"`
	Open   int    `json:"open" esType:"long"`
	Closed int    `json:"closed" esType:"long"`
}

func (repo *Repository) getIssuesAndPullRequests() (*Tickets, *Tickets) {
	return &Tickets{}, &Tickets{}
	log.Print("  getting issues")

	issues := &Tickets{
		Url: fmt.Sprintf("%s/issues", repo.GetHTMLURL()),
	}
	prs := &Tickets{
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
			var s *Tickets
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

type About struct {
	Content     string `json:"content" esType:"text" esAnalyzer:"english"`
	ContentType string `json:"content_type" esType:"keyword"`
}

func (repo *Repository) getReadme() *About {
	files, err := ioutil.ReadDir(repo.clone.Path)
	if err != nil {
		log.Panic(err)
	}

	for _, f := range files {
		m := regexp.MustCompile(`(?i)^readme(?:\.(.+))`).FindStringSubmatch(f.Name())
		if m == nil {
			continue
		}

		contentType := "text/plain"
		if m[1] == "md" {
			contentType = "text/markdown"
		}

		c, err := ioutil.ReadFile(filepath.Join(repo.clone.Path, f.Name()))
		if err != nil {
			log.Panic(err)
		}

		return &About{Content: string(c), ContentType: contentType}
	}

	return nil
}

type Ref struct {
	Name            string     `json:"name" esType:"keyword"`
	IsDefaultBranch bool       `json:"is_head" esType:"boolean"`
	RefType         string     `json:"ref_type" esType:"keyword"`
	LastSeenCommit  string     `json:"last_seen_commit" esType:"keyword"`
	LastUpdated     string     `json:"last_updated" esType:"date"`
	Packages        []*Package `json:"packages"`
}

func (repo *Repository) getRefs() []*Ref {
	refs := []*Ref{repo.newRef(repo.GetDefaultBranch(), true)}

	tags, err := repo.clone.GetTags()
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
	versionTags := make(map[*version.Version]string)
	for _, tag := range tags {
		if !re.MatchString(tag) {
			// log.Printf("  %s does not match", ref.Name().Short())
			continue
		}

		name := tag
		if repo.isGoCore {
			// The version package doesn't like the go core repo's tag names
			// like "go1.0.1".
			name = strings.Replace(name, "go", "", 1)
		}
		v := version.Must(version.NewVersion(name))
		versions = append(versions, v)
		versionTags[v] = tag
	}

	sort.Sort(versions)
	i := 0
	for _, v := range versions {
		// XXX - temporarily only index 3 tags
		if i >= 3 {
			break
		}
		i++
		// log.Printf("  %s matches", ref.Name().Short())
		refs = append(refs, repo.newRef(versionTags[v], false))
	}

	return refs
}

// Mostly copied from git.Repository.GetBranches, but altered to get remote
// branches rather than local.
func (repo *Repository) allBranches() []string {
	prefix := "refs/remotes/origin/"
	stdout, err := git.NewCommand("for-each-ref", "--format=%(refname)", prefix).RunInDir(repo.clone.Path)
	if err != nil {
		log.Panic(err)
	}

	refs := strings.Split(stdout, "\n")

	var branches []string
	// The last item will be an empty string.
	for _, ref := range refs[:len(refs)-1] {
		b := strings.TrimPrefix(ref, prefix)
		if b == "HEAD" {
			continue
		}
		branches = append(branches, b)
	}

	return branches
}

func (repo *Repository) newRef(name string, isBranch bool) *Ref {
	log.Printf("   ref = %s", name)

	if isBranch {
		_, err := git.NewCommand("fetch", "origin", name).RunInDir(repo.clone.Path)
		if err != nil {
			log.Panic(err)
		}
	}

	coName := name
	if isBranch {
		coName = "origin/" + name
	}
	// Despite the reference to Branch this works with any name that git can
	// resolve to a commit.
	err := git.Checkout(repo.clone.Path, git.CheckoutOptions{Branch: coName})
	if err != nil {
		log.Panic(err)
	}

	c, err := repo.clone.GetCommit("HEAD")
	if err != nil {
		log.Panic(err)
	}

	t := "tag"
	if isBranch {
		t = "branch"
	}

	return &Ref{
		Name:            name,
		IsDefaultBranch: name == repo.GetDefaultBranch(),
		RefType:         t,
		LastSeenCommit:  c.ID.String(),
		LastUpdated:     c.Author.When.Format(esmodels.DateTimeFormat),
		Packages:        repo.getPackages(name),
	}
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

func (repo *Repository) getPackages(name string) []*Package {
	return repo.walkTreeForPackages(repo.cloneRoot, name)
}

func (repo *Repository) walkTreeForPackages(dir, refName string) []*Package {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Panic(err)
	}

	var p *Package = nil
	var pkgs []*Package

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
			pkgs = append(pkgs, repo.walkTreeForPackages(path, refName)...)
		}

		// If we've already seen a .go file in this directory then we've made
		// the package for the directory.
		if p != nil {
			continue
		}

		if regexp.MustCompile(`\.go$`).MatchString(name) {
			p = repo.packageForDir(dir, refName)
		}
	}

	if p != nil {
		log.Printf("      package = %s", p.ImportPath)
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

func (repo *Repository) packageForDir(d, refName string) *Package {
	// For some reason bpkg.ImportPath is always giving me ".". But what I'm
	// doing here is really gross. There's got to be a proper way to get this
	// working.
	var importPath string
	if repo.isGoCore {
		importPath = regexp.MustCompile(`^.+?/src/pkg/`).ReplaceAllLiteralString(d, "")
	} else {
		importPath = regexp.MustCompile(`^.+?/`+repo.ID).ReplaceAllLiteralString(d, repo.ID)
	}

	pathInRepo := regexp.MustCompile(`^.+?/`+repo.ID).ReplaceAllLiteralString(d, "")
	browseURL := fmt.Sprintf("%s/tree/%s%s", repo.GetHTMLURL(), refName, pathInRepo)
	dir := directory.New(d, importPath, browseURL)
	pkg, err := doc.NewPackage(dir)
	if err != nil {
		// If this is true it means that this packages lives at a different
		// canonical URL. This can happen when a package has a GitHub repo but
		// you should import it via gopkg.in or some other host.
		if _, ok := err.(gosrc.NotFoundError); ok {
			return nil
		}
		log.Panic(err)
	}

	return &Package{
		Name:         pkg.Name,
		ImportPath:   importPath,
		Doc:          pkg.Doc,
		Synopsis:     pkg.Synopsis,
		Errors:       pkg.Errors,
		IsCommand:    pkg.IsCmd,
		Files:        pkg.Files,
		TestFiles:    pkg.TestFiles,
		Imports:      pkg.Imports,
		TestImports:  pkg.TestImports,
		XTestImports: pkg.XTestImports,
		Consts:       pkg.Consts,
		Funcs:        pkg.Funcs,
		Types:        pkg.Types,
		Vars:         pkg.Vars,
		Examples:     pkg.Examples,
		Notes:        pkg.Notes,
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
