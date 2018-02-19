package esmodels

import (
	"encoding/json"
	"log"
	"reflect"
	"strings"

	"github.com/azer/snakecase"
)

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

func MappingForType(v interface{}) *Mapping {
	t := reflect.TypeOf(v)
	return &Mapping{
		Name:       name(t.Name()),
		Properties: propertiesForType(t),
	}
}

func name(n string) string {
	n = strings.TrimPrefix(n, "ES")
	return snakecase.SnakeCase(n)
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
	field := maybeNested(t, f)
	if field.ESType != "" {
		return field
	}

	esType := f.Tag.Get("esType")
	if esType == "" {
		log.Panicf("Type %s has a field with no esType tag: %s (%s)", t.Name(), f.Name, f.Type.Kind())
	}
	field.ESType = esType

	analyzer := f.Tag.Get("esAnalyzer")
	if analyzer != "" {
		field.Analyzer = analyzer
	}

	return field
}

func maybeNested(t reflect.Type, f reflect.StructField) Field {
	var esType string
	var elem reflect.Type
	if f.Type.Kind() == reflect.Slice {
		esType = "nested"
		if f.Type.Elem().Kind() == reflect.Struct {
			elem = f.Type.Elem()
		} else if f.Type.Elem().Kind() == reflect.Ptr {
			elem = f.Type.Elem().Elem()
		}
	}

	if f.Type.Kind() == reflect.Ptr {
		esType = "object"
		elem = f.Type.Elem()
	}

	if f.Type.Kind() == reflect.Struct {
		esType = "object"
		elem = f.Type
	}

	if f.Type.Kind() == reflect.Map {
		return Field{
			ESType: "object",
		}
	}

	if elem != nil {
		return Field{
			ESType:     esType,
			Properties: propertiesForType(elem),
		}
	}

	return Field{}
}

func (m *Mapping) ToJSON() string {
	j := map[string]Properties{"properties": m.Properties}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		log.Panicf("json.Marshal for %s: %s", m.Name, err)
	}
	return string(b)
}
