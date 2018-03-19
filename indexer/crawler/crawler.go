package crawler

import (
	"net/url"
	"time"

	"github.com/autarch/metagodoc/indexer/repository"
)

type Result struct {
	Crawler    Crawler
	Repository repository.Repository
	Exhausted  bool
	Error      error
}

type Crawler interface {
	Name() string
	SleepDuration() time.Duration
	CrawlAll(ch chan *Result)
	CrawlOne(*url.URL) (repository.Repository, error)
}
