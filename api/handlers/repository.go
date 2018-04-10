package handlers

import (
	"github.com/autarch/metagodoc/api/models"
	"github.com/autarch/metagodoc/api/restapi/operations"
	"github.com/autarch/metagodoc/esmodels"

	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
)

func (h *handlers) GetRepository(params operations.GetRepositoryRepositoryParams) middleware.Responder {
	esr, status := h.getRepo(params.Repository)
	if status != 0 {
		return operations.NewGetRepositoryRepositoryDefault(status)
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
			Refs:   refNames(esr.Refs),
			Stars:  int64(esr.Stars),
			Status: esr.Status.String(),
			Vcs:    esr.VCS,
		},
	)
}
