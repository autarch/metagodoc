package indexer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/autarch/metagodoc/indexer/repository"
	"github.com/google/go-github/github"
	"github.com/olivere/elastic"
)

type Indexer struct {
	elastic    *elastic.Client
	github     *github.Client
	httpClient *http.Client
	cacheRoot  string
	ctx        context.Context
}

func New(token string, cacheRoot string, trace bool) *Indexer {
	log.Print("Starting ...")

	funcs := []elastic.ClientOptionFunc{}
	if trace {
		funcs = append(funcs, elastic.SetTraceLog(log.New(os.Stdout, "ES: ", 0)))
	}

	el, err := elastic.NewClient(funcs...)
	if err != nil {
		log.Panicf("NewClient: %s", err)
	}

	httpClient := &http.Client{Transport: NewGHTransport(token)}
	return &Indexer{
		elastic:    el,
		github:     github.NewClient(httpClient),
		httpClient: httpClient,
		cacheRoot:  cacheRoot,
		ctx:        context.Background(),
	}
}

func (idx *Indexer) IndexAll() {
	log.Print("Search repositories where language=go ...")
	result, _, err := idx.github.Search.Repositories(idx.ctx, "language=go", &github.SearchOptions{})
	if err != nil {
		log.Panicf("GitHub search: %s", err)
	}
	log.Printf("Found %d repositories", *result.Total)

	if *result.Total > 0 {
		for _, r := range result.Repositories {
			idx.indexRepo(repository.New(&r, idx.github, idx.httpClient, idx.cacheRoot, idx.ctx))
		}
	} else {
		log.Print("No repos found")
	}
}

var skipList map[string]bool = map[string]bool{
	// A slide deck?
	// "github.com/GoesToEleven/GolangTraining": true,
	// "github.com/golang/go":                   true,
	// "github.com/qiniu/gobook":                true,
	// // A book.
	// "github.com/adonovan/gopl.io": true,
}

func (idx *Indexer) indexRepo(repo *repository.Repository) {
	log.Printf("Indexing %s", repo.ID)

	if skipList[repo.ID] {
		log.Print("  is on the skip list")
		return
	}

	exists, err := idx.elastic.
		Exists().
		Index("metagodoc-repository").
		Type("repository").
		Id(repo.ID).
		Do(idx.ctx)
	if err != nil {
		log.Panicf("Exists: %s", err)
	}

	elURI := fmt.Sprintf("http://localhost:9200/metagodoc-repository/repository/%s", url.PathEscape(repo.ID))
	if exists {
		log.Printf("  already exists at %s?pretty", elURI)
	} else {
		log.Printf("  did not find any repo where the ID is %s", repo.ID)
	}

	_, err = idx.elastic.
		Index().
		Index("metagodoc-repository").
		Type("repository").
		Id(repo.ID).
		BodyJson(repo.ESModel()).
		Do(idx.ctx)
	if err != nil {
		log.Panicf("Index: %s", err)
	}

	log.Printf("  made new repository record at %s?pretty", elURI)
}
