package main

import (
	"log"
	"os"

	"github.com/autarch/metagodoc/env"
	"github.com/autarch/metagodoc/indexer/indexer"
	"github.com/autarch/metagodoc/logger"
)

func main() {
	l, err := logger.New(logger.NewParams{IsProd: env.IsProd()})
	if err != nil {
		log.Fatal(err)
	}
	defer l.Sync()

	err = indexer.New(indexer.NewParams{
		Logger:       l,
		GitHubToken:  env.GitHubToken(),
		CacheRoot:    env.Root(),
		TraceElastic: env.TraceElastic(),
	}).IndexAll()

	if err != nil {
		l.Fatalf("Error creating indexer: %s", err)
	}

	os.Exit(0)
}
