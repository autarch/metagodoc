package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/autarch/metagodoc/api/models"
	"github.com/autarch/metagodoc/api/restapi/operations"
	"github.com/autarch/metagodoc/esmodels"
	"github.com/olivere/elastic"

	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
)

func (h *handlers) GetRepositoryRefPackage(
	params operations.GetRepositoryRepositoryRefRefPackagePackageParams,
) middleware.Responder {

	result, err := h.el.Search("metagodoc-repository", "metagodoc-author").
		Query(elastic.NewTermQuery(params.Query)).
		//		Sort() how to sort by score?
		Do(context.Background())

	if err != nil {
		h.l.Errorf("Error searching elastic: %s", err)
		return operations.NewGetSearchDefault(500)
	}

	if result.TotalHits() == 0 {
		return operations.NewGetSearchDefault(404)
	}

	r, status := results(results.Hits.Hits)
	if status != 0 {
		operations.NewGetSearchDefault(500)
	}

	return operations.NewGetSearchOK().WithPayload(models.SearchResult{Results: r})
}

func items(hits []*elastic.SearchHit) []*models.SearchResultResultsItem {
	var items []*models.SearchResultResultsItems
	for _, h := range hits {
		i := item(h)
		if i == nil {
			return nil, 500
		}
		items = append(items, i)
	}
	return items, 0
}

func item(hit *elastic.SearchHit) *models.SearchResultResultsItem {
	if hit.Index == "metagodoc-repository" {
		if true {
			return repositoryItem(hit)
		} else {
			return packageItem(hit)
		}
	} else {
		return authorItem(hit)
	}
}

func repositoryItem(hit *elastic.SearchHit) *models.SearchResultResultsItem {
	esr := &esmodels.Repository{}
	err = json.Unmarshal(*hit.Source, esr)
	if err != nil {
		h.l.Errorf("Unmarshal: %s", err)
		return nil
	}

	return &SearchResultResultsItems{
		ItemType:   "repository",
		Repository: repository(esr),
		URL:        strfmt.URI(fmt.Sprintf("/repository/%s", esr.Name)),
		Score:      hit.Score,
	}
}

func packageItem(hit *elastic.SearchHit) *models.SearchResultResultsItem {
	esp := &esmodels.Package{}
	err = json.Unmarshal(*hit.Source, esp)
	if err != nil {
		h.l.Errorf("Unmarshal: %s", err)
		return nil
	}

	return &SearchResultResultsItems{
		ItemType: "repository",
		Package:  repository(esp),
		// XX - have to get the repo & ref for the package somehow
		URL:   strfmt.URI(fmt.Sprintf("/repository/%s/ref/%s/package/%s", esp.Name, esp.Name, esp.Name)),
		Score: hit.Score,
	}
}

func authorItem(hit *elastic.SearchHit) *models.SearchResultResultsItem {
	esa := &esmodels.Author{}
	err = json.Unmarshal(*hit.Source, esa)
	if err != nil {
		h.l.Errorf("Unmarshal: %s", err)
		return nil
	}

	return &SearchResultResultsItems{
		ItemType: "repository",
		Author:   author(esa),
		URL:      strfmt.URI(fmt.Sprintf("/author", esa.Name)),
		Score:    hit.Score,
	}
}
