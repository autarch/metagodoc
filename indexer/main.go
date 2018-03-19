package main

import (
	"log"
	"os"

	"github.com/autarch/metagodoc/indexer/indexer"
)

func main() {
	err := indexer.New(indexer.NewParams{
		GitHubToken:  githubToken(),
		CacheRoot:    root(),
		TraceElastic: trace(),
	}).IndexAll()

	if err != nil {
		log.Printf("Error creating indexer: %s", err)
		os.Exit(1)
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
