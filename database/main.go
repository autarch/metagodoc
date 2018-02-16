package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/autarch/metagodoc/esmodels"
	"github.com/olivere/elastic"
)

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
	for _, m := range esmodels.Mappings() {
		idx := d.makeIndex(m.Name)
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

func (d database) putMapping(idx string, m *esmodels.Mapping) {
	log.Printf("Putting mapping for %s", m.Name)
	_, err := d.client.
		PutMapping().
		Index(idx).
		Type(m.Name).
		BodyString(m.ToJSON()).
		Do(context.Background())
	if err != nil {
		log.Panicf("PutMapping: %s", err)
	}
}
