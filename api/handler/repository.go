package handler

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/autarch/metagodoc/api/models"
	"github.com/autarch/metagodoc/api/restapi/operations"
	"github.com/autarch/metagodoc/esmodels"
	"github.com/autarch/metagodoc/indexer/repository"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/olivere/elastic"
)

func GetRepository(params operations.GetRepositoryRepositoryParams) middleware.Responder {
	el, err := elastic.NewClient()
	if err != nil {
		log.Panicf("NewClient: %s", err)
	}

	result, err := el.Get().
		Index("metagodoc-repository").
		Type("repository").
		Id(params.Repository).
		Do(context.Background())
	if err != nil {
		log.Panicf("Get: %s", err)
	}

	if !result.Found {
		return operations.NewGetRepositoryRepositoryDefault(404)
	}

	esr := &repository.ESRepository{}
	err = json.Unmarshal(*result.Source, esr)
	if err != nil {
		log.Panicf("Unmarshal: %s", err)
	}

	return operations.NewGetRepositoryRepositoryOK().WithPayload(
		&models.Repository{
			About: &models.RepositoryAbout{
				Content:     esr.About.Content,
				ContentType: esr.About.ContentType,
			},
			Created:     dt(esr.Created),
			Description: esr.Description,
			Forks:       int64(esr.Forks),
			FullName:    esr.FullName,
			Issues: &models.Issues{
				Closed: int64(esr.Issues.Closed),
				Open:   int64(esr.Issues.Open),
				URL:    strfmt.URI(esr.Issues.URL),
			},
			LastCrawled: dt(esr.LastCrawled),
			LastUpdated: dt(esr.LastUpdated),
			Name:        esr.Name,
			Owner:       esr.Owner,
			PrimaryURL:  strfmt.URI(esr.PrimaryURL),
			PullRequests: &models.Issues{
				Closed: int64(esr.PullRequests.Closed),
				Open:   int64(esr.PullRequests.Open),
				URL:    strfmt.URI(esr.PullRequests.URL),
			},
			Refs:   refItems(esr.Refs),
			Stars:  int64(esr.Stars),
			Status: esr.Status,
			Vcs:    esr.VCS,
		},
	)
}

func dt(val string) strfmt.DateTime {
	t, err := time.Parse(esmodels.DateTimeFormat, val)
	if err != nil {
		log.Panicf("ParseDateTime: %s", err)
	}
	return strfmt.DateTime(t)
}

func refItems(refs []*repository.Ref) []*models.RepositoryRefsItems {
	var items []*models.RepositoryRefsItems
	for _, r := range refs {
		items = append(items, &models.RepositoryRefsItems{
			Name:            r.Name,
			IsDefaultBranch: r.IsDefaultBranch,
			RefType:         r.RefType,
		})
	}
	return items
}
