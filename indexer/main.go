package main

import (
	"log"
	"os"

	"github.com/autarch/metagodoc/indexer/indexer"
)

func main() {
	token := os.Getenv("GOPAL_GITHUB_TOKEN")
	if token == "" {
		log.Fatal("You must set GOPAL_GITHUB_TOKEN")
	}

	i := indexer.New(token, "/home/autarch/.metagodoc-cache", trace())
	i.IndexAll()
}

func trace() bool {
	return os.Getenv("GOPAL_ELASTIC_TRACE") != ""
}
