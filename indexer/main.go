package main

import (
	"log"
	"os"

	"github.com/autarch/metagodoc/indexer/indexer"
	"github.com/autarch/metagodoc/logger"
)

func main() {
	l, err := logger.New(logger.NewParams{})
	if err != nil {
		log.Fatal(err)
	}
	defer l.Sync()

	err = indexer.New(indexer.NewParams{
		Logger:       l,
		GitHubToken:  githubToken(),
		CacheRoot:    root(),
		TraceElastic: trace(),
	}).IndexAll()

	if err != nil {
		l.Fatalf("Error creating indexer: %s", err)
	}

	os.Exit(0)
}

func githubToken() string {
	return os.Getenv("METAGODOC_GITHUB_TOKEN")
}

func root() string {
	root := os.Getenv("METAGODOC_ROOT")
	if root != "" {
		return root
	}

	return "/var/cache/metagodoc"
}

func trace() bool {
	return os.Getenv("METAGODOC_ELASTIC_TRACE") != ""
}
