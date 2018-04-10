package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/autarch/metagodoc/api/models"
	"github.com/autarch/metagodoc/api/restapi/operations"
	"github.com/autarch/metagodoc/esmodels"

	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
)

func (h *handlers) GetRepository(params operations.GetRepositoryRepositoryParams) middleware.Responder {
	result, err := h.el.Get().
		Index("metagodoc-repository").
		Type("repository").
		Id(params.Repository).
		Do(context.Background())
	if err != nil {
		h.l.Errorf("Elastic get failed: %s", err)
		return operations.NewGetRepositoryRepositoryDefault(500)
	}

	if !result.Found {
		return operations.NewGetRepositoryRepositoryDefault(404)
	}

	esr := &esmodels.Repository{}
	err = json.Unmarshal(*result.Source, esr)
	if err != nil {
		log.Panicf("Unmarshal: %s", err)
	}

	return h.maybeRepositoryOkResponse(esr)
}

func (h *handlers) maybeRepositoryOkResponse(esr *esmodels.Repository) middleware.Responder {
	c, err := h.dt(esr.Created)
	if err != nil {
		return operations.NewGetRepositoryRepositoryDefault(500)
	}

	lc, err := h.dt(esr.LastCrawled)
	if err != nil {
		return operations.NewGetRepositoryRepositoryDefault(500)
	}

	lu, err := h.dt(esr.LastUpdated)
	if err != nil {
		return operations.NewGetRepositoryRepositoryDefault(500)
	}

	return operations.NewGetRepositoryRepositoryOK().WithPayload(
		&models.Repository{
			About: &models.RepositoryAbout{
				Content:     esr.About.Content,
				ContentType: esr.About.ContentType,
			},
			Created:     *c,
			Description: esr.Description,
			Forks:       int64(esr.Forks),
			FullName:    esr.FullName,
			Issues: &models.Issues{
				Closed: int64(esr.Issues.Closed),
				Open:   int64(esr.Issues.Open),
				URL:    strfmt.URI(esr.Issues.URL),
			},
			LastCrawled: *lc,
			LastUpdated: *lu,
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
			Status: esr.Status.String(),
			Vcs:    esr.VCS,
		},
	)
}

func refItems(refs []*esmodels.Ref) []string {
	var items []string
	for _, r := range refs {
		items = append(items, r.Name)
	}
	return items
}
