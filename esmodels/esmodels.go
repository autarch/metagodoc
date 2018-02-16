package esmodels

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/azer/snakecase"
)

type Repository struct {
	Name         string   `json:"name" esType:"keyword"`
	FullName     string   `json:"full_name" esType:"keyword"`
	Description  string   `json:"description" esType:"text" esAnalyzer:"english"`
	VCS          string   `json:"vcs" esType:"keyword"`
	PrimaryURL   string   `json:"primary_url" esType:"keyword"`
	Issues       *Tickets `json:"issues"`
	PullRequests *Tickets `json:"pull_requests"`
	Owner        string   `json:"owner" esType:"keyword"`
	Created      string   `json:"created" esType:"date"`
	LastUpdated  string   `json:"last_updated" esType:"date"`
	LastCrawled  string   `json:"last_crawled" esType:"date"`
	Stars        int      `json:"stars" esType:"long"`
	Forks        int      `json:"forks" esType:"long"`
	IsFork       bool     `json:"is_fork" esType:"boolean"`
	Status       string   `json:"status" esType:"keyword"`
	About        *About   `json:"about""`
	Refs         []*Ref   `json:"refs"`
}

type Tickets struct {
	Url    string `json:"url" esType:"keyword"`
	Open   int    `json:"open" esType:"long"`
	Closed int    `json:"closed" esType:"long"`
}

type About struct {
	Content     string `json:"content" esType:"text" esAnalyzer:"english"`
	ContentType string `json:"content_type" esType:"keyword"`
}

type Ref struct {
	Name            string     `json:"name" esType:"keyword"`
	IsDefaultBranch bool       `json:"is_head" esType:"boolean"`
	RefType         string     `json:"ref_type" esType:"keyword"`
	LastSeenCommit  string     `json:"last_seen_commit" esType:"keyword"`
	LastUpdated     string     `json:"last_updated" esType:"date"`
	Packages        []*Package `json:"packages"`
}

type Package struct {
	Name         string   `json:"name" esType:"keyword"`
	ImportPath   string   `json:"import_path" esType:"keyword"`
	Synopsis     string   `json:"synopsis" esType:"text" esAnalyzer:"english"`
	Errors       []string `json:"errors" esType:"keyword"`
	IsCommand    bool     `json:"is_command" esType:"boolean"`
	Files        []string `json:"files" esType:"keyword"`
	TestFiles    []string `json:"test_files" esType:"keyword"`
	XTestFiles   []string `json:"x_test_files" esType:"keyword"`
	Imports      []string `json:"imports" esType:"keyword"`
	TestImports  []string `json:"test_imports" esType:"keyword"`
	XTestImports []string `json:"x_test_imports" esType:"keyword"`
}

type Author struct {
	Name         string   `json:"name" esType:"keyword"`
	PrimaryURL   string   `json:"primary_url" esType:"keyword"`
	Created      string   `json:"created" esType:"date"`
	LastUpdated  string   `json:"last_updated" esType:"date"`
	Repositories []string `json:"name" esType:"keyword"`
}

const DateTimeFormat = "2006-01-02T15:04:05"

type Mapping struct {
	Name       string
	Properties Properties
}

type Field struct {
	ESType     string     `json:"type"`
	Analyzer   string     `json:"analyzer,omitempty"`
	Properties Properties `json:"properties,omitempty"`
}

type Properties map[string]Field

func Mappings() []*Mapping {
	return []*Mapping{
		mappingForType(Repository{}),
		mappingForType(Author{}),
	}
}

func mappingForType(v interface{}) *Mapping {
	t := reflect.TypeOf(v)
	if t.PkgPath() != reflect.TypeOf(Mapping{}).PkgPath() {
		log.Panicf("Cannot make a mapping for types from %s", t.PkgPath())
	}

	return &Mapping{
		Name:       snakecase.SnakeCase(t.Name()),
		Properties: propertiesForType(t),
	}
}

func propertiesForType(t reflect.Type) Properties {
	props := Properties{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		props[snakecase.SnakeCase(f.Name)] = esField(t, f)
	}
	return props
}

func esField(t reflect.Type, f reflect.StructField) Field {
	if f.Type.Kind() == reflect.Ptr || (f.Type.Kind() == reflect.Slice && f.Type.Elem().Kind() == reflect.Ptr) {
		elem := f.Type.Elem()
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		return Field{
			ESType:     "nested",
			Properties: propertiesForType(elem),
		}
	}
	esType := f.Tag.Get("esType")
	if esType == "" {
		log.Panicf("Type %s has a field with no esType tag: %s (%s)", t.Name(), f.Name, f.Type.Kind())
	}
	field := Field{ESType: esType}

	analyzer := f.Tag.Get("esAnalyzer")
	if analyzer != "" {
		field.Analyzer = analyzer
	}

	return field
}

func (m *Mapping) ToJSON() string {
	j := map[string]Properties{"properties": m.Properties}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		log.Panicf("json.Marshal for %s: %s", m.Name, err)
	}
	return string(b)
}
