package crawler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/autarch/metagodoc/indexer/repository"
	"github.com/autarch/metagodoc/logger"
	"github.com/google/go-github/github"
	"github.com/hashicorp/errwrap"
	"golang.org/x/oauth2"
)

type githubCrawler struct {
	l             *log.Logger
	cacheRoot     string
	github        *github.Client
	currentResult *github.RepositoriesSearchResult
	currentIdx    int
	nextPage      int
	ctx           context.Context
}

func NewGitHubCrawler(cacheRoot string, token string, ctx context.Context) (Crawler, error) {
	if token == "" {
		return nil, errors.New("Cannot crawl GitHub with an access token")
	}

	l := logger.New("GitHub Crawler", true)

	return &githubCrawler{
		l:         l,
		cacheRoot: cacheRoot,
		github:    githubClient(token),
		nextPage:  1,
		ctx:       ctx,
	}, nil
}

func githubClient(token string) *github.Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

type githubTransport struct {
	token string
	*http.Transport
}

func (t *githubTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	req.SetBasicAuth(t.token, "x-oauth-basic")
	return t.Transport.RoundTrip(req)
}

func (gh *githubCrawler) Name() string {
	return "GitHub"
}

func (gh *githubCrawler) SleepDuration() time.Duration {
	return time.Duration(15) * time.Minute
}

func (gh *githubCrawler) CrawlAll(ch chan *Result) {
	for true {
		more := gh.crawlNextPage(ch)
		if !more {
			break
		}
	}
}

func (gh *githubCrawler) crawlNextPage(ch chan *Result) bool {
	if gh.nextPage == 0 {
		ch <- gh.newResult(nil, nil, true)
		return false
	}

	result, err := gh.getNextPage()
	if err != nil {
		// This should end up putting the crawler to sleep.
		ch <- gh.newResult(nil, err, false)
		return false
	}

	for _, r := range result.Repositories {
		ghRepo, err := repository.NewGitHubRepository(
			&r,
			gh.github,
			gh.cacheRoot,
			gh.ctx,
		)
		// If the repo was not crawled but there is no error (for example
		// because it was skipped intentionally) then there's no result to
		// send.
		if ghRepo != nil || err != nil {
			ch <- gh.newResult(ghRepo, err, false)
		}
	}

	return true
}

func (gh *githubCrawler) newResult(r repository.Repository, err error, ex bool) *Result {
	return &Result{Crawler: gh, Repository: r, Error: err, Exhausted: ex}
}

func (gh *githubCrawler) getNextPage() (*github.RepositoriesSearchResult, error) {
	gh.l.Printf("Searching for repositories where language=go, page %d", gh.nextPage)
	result, resp, err := gh.github.Search.Repositories(
		gh.ctx,
		"language=go",
		&github.SearchOptions{ListOptions: github.ListOptions{Page: gh.nextPage}},
	)
	if err != nil {
		return nil, errwrap.Wrapf("GitHub search error: {{err}}", err)
	}

	if result.GetTotal() == 0 {
		gh.l.Print("Did not find any GitHub repositories on page %d", gh.nextPage)
		return nil, nil
	}

	gh.l.Printf("Found %d repositories", result.GetTotal())
	gh.nextPage = resp.NextPage

	return result, nil
}

func (gh *githubCrawler) CrawlOne(*url.URL) (repository.Repository, error) {
	return nil, nil
}
