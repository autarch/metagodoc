package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/olivere/elastic"
)

type mapping struct {
	name       string
	properties properties
}

type field struct {
	ESType     string     `json:"type"`
	Analyzer   string     `json:"analyzer,omitempty"`
	Properties properties `json:"properties,omitempty"`
}

type properties map[string]field

var repoMapping = mapping{
	"repository",
	properties{
		"name":        field{ESType: "keyword"},
		"full_name":   field{ESType: "keyword"},
		"vcs":         field{ESType: "keyword"},
		"primary_url": field{ESType: "keyword"},
		"issues": field{
			ESType: "nested",
			Properties: properties{
				"url":    field{ESType: "keyword"},
				"open":   field{ESType: "long"},
				"closed": field{ESType: "long"},
			},
		},
		"pull_requests": field{
			ESType: "nested",
			Properties: properties{
				"url":    field{ESType: "keyword"},
				"open":   field{ESType: "long"},
				"closed": field{ESType: "long"},
			},
		},
		"owner":        field{ESType: "keyword"},
		"created":      field{ESType: "date"},
		"last_updated": field{ESType: "date"},
		"last_crawled": field{ESType: "date"},
		"stars":        field{ESType: "long"},
		"forks":        field{ESType: "long"},
		"is_fork":      field{ESType: "boolean"},
		"status":       field{ESType: "keyword"},
		"about": field{
			ESType: "nested",
			Properties: properties{
				"content": field{
					ESType:   "text",
					Analyzer: "english",
				},
				"content_type": field{ESType: "keyword"}},
		},
		"refs": field{
			ESType: "nested",
			Properties: properties{
				"name":             field{ESType: "keyword"},
				"is_head":          field{ESType: "boolean"},
				"last_seen_commit": field{ESType: "keyword"},
				"packages": field{
					ESType: "nested",
					Properties: properties{
						"name":           field{ESType: "keyword"},
						"import_path":    field{ESType: "keyword"},
						"is_command":     field{ESType: "boolean"},
						"files":          field{ESType: "keyword"},
						"test_files":     field{ESType: "keyword"},
						"x_test_files":   field{ESType: "keyword"},
						"imports":        field{ESType: "keyword"},
						"test_imports":   field{ESType: "keyword"},
						"x_test_imports": field{ESType: "keyword"},
					},
				},
			},
		},
	},
}

var authorMapping = mapping{
	"author",
	properties{
		"name":         field{ESType: "keyword"},
		"primary_url":  field{ESType: "keyword"},
		"author":       field{ESType: "keyword"},
		"created":      field{ESType: "date"},
		"last_updated": field{ESType: "date"},
		"repositories": field{ESType: "keyword"},
	},
}

var mappings = []mapping{repoMapping, authorMapping}

type database struct {
	client *elastic.Client
}

func main() {
	d := New(true)
	d.makeIndices()
}

func New(trace bool) database {
	funcs := []elastic.ClientOptionFunc{}
	if trace {
		funcs = append(funcs, elastic.SetTraceLog(log.New(os.Stdout, "ES: ", 0)))
	}

	client, err := elastic.NewClient(funcs...)
	if err != nil {
		log.Panicf("NewClient: %s", err)
	}

	return database{
		client: client,
	}
}

func (d database) makeIndices() {
	for _, m := range mappings {
		idx := d.makeIndex(m.name)
		d.putMapping(idx, m)
	}
}

func (d database) makeIndex(m string) string {
	name := fmt.Sprintf("metagodoc-%s", m)

	exists, err := d.client.IndexExists(name).Do(context.Background())
	if err != nil {
		log.Panicf("IndexExists: %s", err)
	}

	if exists {
		_, err := d.client.DeleteIndex(name).Do(context.Background())
		if err != nil {
			log.Panicf("DeleteIndex: %s", err)
		}
	}

	_, err = d.client.CreateIndex(name).Do(context.Background())
	if err != nil {
		log.Panicf("CreateIndex: %s", err)
	}

	return name
}

func (d database) putMapping(idx string, m mapping) {
	log.Printf("Putting mapping for %s", m.name)
	_, err := d.client.
		PutMapping().
		Index(idx).
		Type(m.name).
		BodyString(mappingToJSON(m)).
		Do(context.Background())
	if err != nil {
		log.Panicf("PutMapping: %s", err)
	}
}

func mappingToJSON(m mapping) string {
	j := map[string]properties{"properties": m.properties}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		log.Panicf("json.Marshal for %s: %s", m.name, err)
	}
	return string(b)
}
