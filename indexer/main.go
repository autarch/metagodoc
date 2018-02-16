package main

import (
	"log"
	"os"

	"github.com/autarch/metagodoc/indexer/indexer"
)

func main() {
	token := os.Getenv("MG_GITHUB_TOKEN")
	if token == "" {
		log.Fatal("You must set MG_GITHUB_TOKEN")
	}

	i := indexer.New(token, "/home/autarch/.metagodoc-cache", trace())
	i.IndexAll()
}

func trace() bool {
	return os.Getenv("MG_ELASTIC_TRACE") != ""
}
