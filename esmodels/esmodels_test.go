package esmodels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMappings(t *testing.T) {
	mappings := Mappings()

	repository := &Mapping{
		"repository",
		Properties{
			"name":        Field{ESType: "keyword"},
			"full_name":   Field{ESType: "keyword"},
			"vcs":         Field{ESType: "keyword"},
			"primary_url": Field{ESType: "keyword"},
			"issues": Field{
				ESType: "nested",
				Properties: Properties{
					"url":    Field{ESType: "keyword"},
					"open":   Field{ESType: "long"},
					"closed": Field{ESType: "long"},
				},
			},
			"pull_requests": Field{
				ESType: "nested",
				Properties: Properties{
					"url":    Field{ESType: "keyword"},
					"open":   Field{ESType: "long"},
					"closed": Field{ESType: "long"},
				},
			},
			"owner":        Field{ESType: "keyword"},
			"created":      Field{ESType: "date"},
			"last_updated": Field{ESType: "date"},
			"last_crawled": Field{ESType: "date"},
			"stars":        Field{ESType: "long"},
			"forks":        Field{ESType: "long"},
			"is_fork":      Field{ESType: "boolean"},
			"status":       Field{ESType: "keyword"},
			"about": Field{
				ESType: "nested",
				Properties: Properties{
					"content": Field{
						ESType:   "text",
						Analyzer: "english",
					},
					"content_type": Field{ESType: "keyword"}},
			},
			"refs": Field{
				ESType: "nested",
				Properties: Properties{
					"name":              Field{ESType: "keyword"},
					"is_default_branch": Field{ESType: "boolean"},
					"ref_type":          Field{ESType: "keyword"},
					"last_seen_commit":  Field{ESType: "date"},
					"last_updated":      Field{ESType: "date"},
					"packages": Field{
						ESType: "nested",
						Properties: Properties{
							"name":        Field{ESType: "keyword"},
							"import_path": Field{ESType: "keyword"},
							"synopsis": Field{
								ESType:   "text",
								Analyzer: "english",
							},
							"errors":         Field{ESType: "keyword"},
							"is_command":     Field{ESType: "boolean"},
							"files":          Field{ESType: "keyword"},
							"test_files":     Field{ESType: "keyword"},
							"x_test_files":   Field{ESType: "keyword"},
							"imports":        Field{ESType: "keyword"},
							"test_imports":   Field{ESType: "keyword"},
							"x_test_imports": Field{ESType: "keyword"},
						},
					},
				},
			},
		},
	}
	assert.Equal(t, repository, mappings[0], "repository mapping is correct")

	author := &Mapping{
		"author",
		Properties{
			"name":         Field{ESType: "keyword"},
			"primary_url":  Field{ESType: "keyword"},
			"created":      Field{ESType: "date"},
			"last_updated": Field{ESType: "date"},
			"repositories": Field{ESType: "keyword"},
		},
	}
	assert.Equal(t, author, mappings[1], "author mapping is correct")
}
