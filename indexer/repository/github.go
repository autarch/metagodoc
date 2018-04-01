package repository

import (
	"container/list"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/autarch/metagodoc/esmodels"
	"github.com/autarch/metagodoc/indexer/directory"
	"github.com/autarch/metagodoc/indexer/doc"
	"github.com/autarch/metagodoc/logger"

	"code.gitea.io/git"
	"github.com/golang/gddo/gosrc"
	"github.com/google/go-github/github"
	version "github.com/hashicorp/go-version"
)

type githubRepository struct {
	l            *log.Logger
	githubRepo   *github.Repository
	githubClient *github.Client
	clone        *git.Repository
	ctx          context.Context
	isGoCore     bool
	cloneRoot    string

	// A unique ID for the repository based on its URL without the scheme. So
	// for a GitHub repo like "https://github.com/stretchr/testify" this would
	// be "github.com/stretchr/testify". This may be turned into import paths
	// for individual packages.
	id string

	// Version control system: git, hg, bzr, ...
	VCS esmodels.VCSType
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

func NewGitHubRepository(
	ghr *github.Repository,
	github *github.Client,
	cacheRoot string,
	ctx context.Context,
) (*githubRepository, error) {

	id := regexp.MustCompile(`^https?://`).ReplaceAllString(ghr.GetHTMLURL(), "")

	prefix := regexp.MustCompile(`^github\.com`).ReplaceAllString(id, "@GH")
	l := logger.New(prefix, true)

	l.Printf("Indexing %s", id)

	if skipList[id] {
		l.Print("  is on the skip list")
		return nil, nil
	}

	isGoCore := id == "github.com/golang/go"
	repo := &githubRepository{
		l:            l,
		githubRepo:   ghr,
		githubClient: github,
		ctx:          ctx,
		isGoCore:     isGoCore,
		cloneRoot:    filepath.Join(cacheRoot, "repos", id),
		id:           id,
		VCS:          esmodels.Git,
	}
	repo.clone = repo.getGitRepo()

	return repo, nil
}

func (repo *githubRepository) ESModel() *esmodels.Repository {
	issues, prs := repo.getIssuesAndPullRequests()
	return &esmodels.Repository{
		Name:         repo.githubRepo.GetName(),
		FullName:     repo.githubRepo.GetFullName(),
		VCS:          string(repo.VCS),
		Description:  repo.githubRepo.GetDescription(),
		PrimaryURL:   repo.githubRepo.GetHTMLURL(),
		Issues:       issues,
		PullRequests: prs,
		Owner:        repo.githubRepo.GetOwner().GetLogin(),
		Created:      repo.githubRepo.GetCreatedAt().UTC().Format(esmodels.DateTimeFormat),
		LastUpdated:  repo.githubRepo.GetPushedAt().Format(esmodels.DateTimeFormat),
		LastCrawled:  time.Now().UTC().Format(esmodels.DateTimeFormat),
		Stars:        repo.githubRepo.GetStargazersCount(),
		Forks:        repo.githubRepo.GetForksCount(),
		Status:       repo.getStatus(),
		About:        repo.getReadme(),
		IsFork:       repo.githubRepo.GetFork(),
		Refs:         repo.getRefs(),
	}
}

func (repo *githubRepository) ID() string {
	return repo.id
}

func (repo *githubRepository) getGitRepo() *git.Repository {
	var c *git.Repository

	exists := pathExists(repo.cloneRoot)
	if !exists {
		repo.l.Printf("  %s does not exist at %s - cloning", repo.id, repo.cloneRoot)
		err := git.Clone(repo.githubRepo.GetCloneURL(), repo.cloneRoot, git.CloneRepoOptions{})
		if err != nil {
			repo.l.Panic(err)
		}
	}

	var err error
	c, err = git.OpenRepository(repo.cloneRoot)
	if err != nil {
		repo.l.Panic(err)
	}

	if exists {
		repo.l.Printf("  %s exists at %s - fetching", repo.id, repo.cloneRoot)
		_, err = git.NewCommand("fetch", "--tags").RunInDir(c.Path)
		if err != nil {
			repo.l.Panic(err)
		}
	}

	return c
}

// A repository with no commits within the last 2 years will be considered
// inactive. But if another active repo imports this one then we will consider
// this one active.
const twoYears = 2 * 365 * 24 * time.Hour

func (repo *githubRepository) getStatus() esmodels.ActivityStatus {
	head, err := repo.clone.GetBranchCommit(repo.githubRepo.GetDefaultBranch())
	if err != nil {
		repo.l.Panic(err)
	}

	if time.Now().Sub(head.Author.When) > twoYears {
		return esmodels.NoRecentCommits
	}

	commits, err := head.CommitsBeforeLimit(2)
	if err != nil {
		repo.l.Panic(err)
	}
	commits.PushFront(head)

	if repo.githubRepo.GetFork() {
		if repo.githubRepo.GetPushedAt().Before(repo.githubRepo.GetCreatedAt().Time) {
			return esmodels.DeadEndFork
		} else if repo.isQuickFork(commits) {
			return esmodels.QuickFork
		}
	}

	return esmodels.Active
}

const oneWeek = 7 * 24 * time.Hour

// isQuickFork reports whether the repository is a "quick fork": it has fewer
// than 3 commits, all within a week of the repo creation, createdAt.  Commits
// must be in reverse chronological order by Commit.Committer.Date.
func (repo *githubRepository) isQuickFork(firstThree *list.List) bool {
	oneWeekOld := repo.githubRepo.GetCreatedAt().Add(oneWeek)
	if oneWeekOld.After(time.Now()) {
		return false // a newborn baby of a repository
	}
	for e := firstThree.Front(); e != nil; e = e.Next() {
		c := e.Value.(*git.Commit)
		if c.Author.When.After(oneWeekOld) {
			return false
		}
		if c.Author.When.Before(repo.githubRepo.GetCreatedAt().Time) {
			break
		}
	}
	return true
}

func (repo *githubRepository) getIssuesAndPullRequests() (*esmodels.Tickets, *esmodels.Tickets) {
	repo.l.Print("  getting issues")

	issues := &esmodels.Tickets{
		URL: fmt.Sprintf("%s/issues", repo.githubRepo.GetHTMLURL()),
	}
	prs := &esmodels.Tickets{
		URL: fmt.Sprintf("%s/pulls", repo.githubRepo.GetHTMLURL()),
	}

	opts := &github.IssueListByRepoOptions{State: "all"}
	for {
		issuesList, resp, err := repo.githubClient.Issues.ListByRepo(
			repo.ctx,
			repo.githubRepo.GetOwner().GetLogin(),
			repo.githubRepo.GetName(),
			opts,
		)
		if err != nil {
			repo.l.Panic(err)
		}

		for _, i := range issuesList {
			var s *esmodels.Tickets
			if i.IsPullRequest() {
				s = prs
			} else {
				s = issues
			}
			if i.GetClosedAt().IsZero() {
				s.Open++
			} else {
				s.Closed++
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return issues, prs
}

func (repo *githubRepository) getReadme() *esmodels.About {
	files, err := ioutil.ReadDir(repo.clone.Path)
	if err != nil {
		repo.l.Panic(err)
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
			repo.l.Panic(err)
		}

		return &esmodels.About{Content: string(c), ContentType: contentType}
	}

	return nil
}

func (repo *githubRepository) getRefs() []*esmodels.Ref {
	refs := []*esmodels.Ref{repo.newRef(repo.githubRepo.GetDefaultBranch(), true)}

	tags, err := repo.clone.GetTags()
	if err != nil {
		repo.l.Panic(err)
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
			// repo.l.Printf("  %s does not match", ref.Name().Short())
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
		// repo.l.Printf("  %s matches", ref.Name().Short())
		refs = append(refs, repo.newRef(versionTags[v], false))
	}

	return refs
}

// Mostly copied from git.Repository.GetBranches, but altered to get remote
// branches rather than local.
func (repo *githubRepository) allBranches() []string {
	prefix := "refs/remotes/origin/"
	stdout, err := git.NewCommand("for-each-ref", "--format=%(refname)", prefix).RunInDir(repo.clone.Path)
	if err != nil {
		repo.l.Panic(err)
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

func (repo *githubRepository) newRef(name string, isBranch bool) *esmodels.Ref {
	repo.l.Printf("   ref = %s", name)

	if isBranch {
		_, err := git.NewCommand("fetch", "origin", name).RunInDir(repo.clone.Path)
		if err != nil {
			repo.l.Panic(err)
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
		repo.l.Panic(err)
	}

	c, err := repo.clone.GetCommit("HEAD")
	if err != nil {
		repo.l.Panic(err)
	}

	t := "tag"
	if isBranch {
		t = "branch"
	}

	return &esmodels.Ref{
		Name:            name,
		IsDefaultBranch: name == repo.githubRepo.GetDefaultBranch(),
		RefType:         t,
		LastSeenCommit:  c.ID.String(),
		LastUpdated:     c.Author.When.Format(esmodels.DateTimeFormat),
		Packages:        repo.getPackages(name),
	}
}

func (repo *githubRepository) getPackages(name string) []*esmodels.Package {
	return repo.walkTreeForPackages(repo.cloneRoot, name)
}

func (repo *githubRepository) walkTreeForPackages(dir, refName string) []*esmodels.Package {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		repo.l.Panic(err)
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
		repo.l.Printf("      package = %s", p.ImportPath)
		return append(pkgs, p)
	}
	return pkgs
}

// There are paths that contain go code in the golang/go repo that are not
// organized in valid manner, for example
// https://github.com/golang/go/tree/master/doc/progs, which contains a bunch
// of example programs, each with its own package.
func (repo *githubRepository) isGoCorePackage(path string) bool {
	importPath := strings.Replace(path, repo.cloneRoot+"/src", "", 1)
	return pathFlags[importPath]&packagePath != 0
}

func (repo *githubRepository) packageForDir(d, refName string) *esmodels.Package {
	// For some reason bpkg.ImportPath is always giving me ".". But what I'm
	// doing here is really gross. There's got to be a proper way to get this
	// working.
	var importPath string
	if repo.isGoCore {
		importPath = regexp.MustCompile(`^.+?/src/pkg/`).ReplaceAllLiteralString(d, "")
	} else {
		importPath = regexp.MustCompile(`^.+?/`+repo.id).ReplaceAllLiteralString(d, repo.id)
	}

	pathInRepo := regexp.MustCompile(`^.+?/`+repo.id).ReplaceAllLiteralString(d, "")
	browseURL := fmt.Sprintf("%s/tree/%s%s", repo.githubRepo.GetHTMLURL(), refName, pathInRepo)
	dir := directory.New(d, importPath, browseURL)
	pkg, err := doc.NewPackage(dir)
	if err != nil {
		// If this is true it means that this packages lives at a different
		// canonical URL. This can happen when a package has a GitHub repo but
		// you should import it via gopkg.in or some other host.
		if _, ok := err.(gosrc.NotFoundError); ok {
			return nil
		}
		repo.l.Panic(err)
	}

	return &esmodels.Package{
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
