package indexer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/autarch/metagodoc/indexer/crawler"
	"github.com/autarch/metagodoc/indexer/repository"
	"github.com/autarch/metagodoc/logger"

	"github.com/hako/durafmt"
	"github.com/hashicorp/errwrap"
	"github.com/olivere/elastic"
)

type NewParams struct {
	Logger       *logger.Logger
	GitHubToken  string
	CacheRoot    string
	TraceElastic bool
}

type crawlers struct {
	available []crawler.Crawler
	sleeping  map[crawler.Crawler]time.Time
}

type Indexer struct {
	l           *logger.Logger
	elastic     *elastic.Client
	cacheRoot   string
	githubToken string
	crawlers    crawlers
	ctx         context.Context
	err         error
}

func New(p NewParams) *Indexer {
	funcs := []elastic.ClientOptionFunc{}
	if p.TraceElastic {
		funcs = append(funcs, elastic.SetTraceLog(p.Logger))
	}

	el, err := elastic.NewClient(funcs...)
	if err != nil {
		return &Indexer{err: err}
	}

	info, err := os.Stat(p.CacheRoot)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(p.CacheRoot, 0755)
			if err != nil {
				return &Indexer{err: errwrap.Wrapf(fmt.Sprintf("Could not create %s directory: {{err}}", p.CacheRoot), err)}
			}
		} else {
			return &Indexer{err: errwrap.Wrapf(fmt.Sprintf("Could not stat %s: {{err}}", p.CacheRoot), err)}
		}
	} else if !info.IsDir() {
		return &Indexer{err: fmt.Errorf("The root that was passed, %s, is not a directory", p.CacheRoot)}
	}

	c := context.Background()
	idx := &Indexer{
		l:           p.Logger,
		elastic:     el,
		cacheRoot:   p.CacheRoot,
		githubToken: p.GitHubToken,
		ctx:         c,
	}

	idx.setCrawlers()

	return idx
}

func (idx *Indexer) setCrawlers() {
	gh, err := crawler.NewGitHubCrawler(idx.l, idx.cacheRoot, idx.githubToken, idx.ctx)
	if err != nil {
		idx.err = err
		return
	}
	idx.crawlers.available = append(idx.crawlers.available, gh)
}

func (idx *Indexer) IndexAll() error {
	if idx.err != nil {
		return idx.err
	}

	ch := make(chan *crawler.Result)
	defer close(ch)
	for true {
		idx.loop(ch)
	}

	return nil
}

func (idx *Indexer) loop(ch chan *crawler.Result) {
	idx.maybeWakeCrawlers()

	if len(idx.crawlers.available) == 0 {
		until := idx.untilNextWake()
		idx.l.Infof("Sleeping for %s", durafmt.Parse(until))
		time.Sleep(until)
	}

	available := idx.crawlers.available
	idx.crawlers.available = []crawler.Crawler{}
	idx.l.Info("Starting all available crawlers")
	for _, c := range available {
		idx.l.Infof("Starting %s crawler", c.Name())
		go c.CrawlAll(ch)
	}

	// We want result handling in its own goroutine so we can wake up sleeping
	// crawlers on time without waiting for a result from the channel.
	go func() {
		for r := range ch {
			idx.l.Infof("Got a result from the %s crawler", r.Crawler.Name())
			if r.Error != nil {
				if r.Exhausted {
					idx.putCrawlerToSleep(r.Crawler)
				} else {
					idx.l.Infof("%s crawler returned an error: %s", r.Crawler.Name(), r.Error)
					idx.putCrawlerToSleep(r.Crawler)
				}
				continue
			}

			go idx.indexRepo(r.Repository)
		}
	}()
}

func (idx *Indexer) maybeWakeCrawlers() {
	now := time.Now()
	for c, t := range idx.crawlers.sleeping {
		if t.After(now) {
			continue
		}
		idx.l.Infof("Waking %s crawler", c.Name())
		delete(idx.crawlers.sleeping, c)
		idx.crawlers.available = append(idx.crawlers.available, c)
	}
}

func (idx *Indexer) untilNextWake() time.Duration {
	var durs []time.Duration
	now := time.Now()
	for _, t := range idx.crawlers.sleeping {
		durs = append(durs, t.Sub(now))
	}

	// If there are no crawlers sleeping that means all crawlers are currently
	// available. We will sleep for a minute and then try again.
	if len(durs) == 0 {
		return time.Duration(1) * time.Minute
	}

	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	return durs[0]
}

func (idx *Indexer) putCrawlerToSleep(c crawler.Crawler) {
	dur := c.SleepDuration()
	wake := time.Now().Add(dur)
	idx.l.Infof("Putting %s crawler to sleep for %s, will wake at %s",
		c.Name(),
		durafmt.Parse(dur),
		wake.Format("2006-01-02 15:04:05"),
	)

	var available []crawler.Crawler
	for _, a := range idx.crawlers.available {
		if c == a {
			idx.crawlers.sleeping[a] = wake
			continue
		}
		available = append(available, a)
	}

	idx.crawlers.available = available
}

func (idx *Indexer) indexRepo(repo repository.Repository) {
	// Repo is being intentionally skipped.
	if repo == nil {
		return
	}

	exists, err := idx.elastic.
		Exists().
		Index("metagodoc-repository").
		Type("repository").
		Id(repo.ID()).
		Do(idx.ctx)
	if err != nil {
		idx.l.Panicf("Exists: %s", err)
	}

	elURI := fmt.Sprintf("http://localhost:9200/metagodoc-repository/repository/%s", url.PathEscape(repo.ID()))
	if exists {
		idx.l.Infof("  already exists at %s?pretty", elURI)
	} else {
		idx.l.Infof("  did not find any repo where the ID is %s", repo.ID())
	}

	_, err = idx.elastic.
		Index().
		Index("metagodoc-repository").
		Type("repository").
		Id(repo.ID()).
		BodyJson(repo.ESModel()).
		Do(idx.ctx)
	if err != nil {
		idx.l.Panicf("Index: %s", err)
	}

	idx.l.Infof("  made new repository record at %s?pretty", elURI)
}
