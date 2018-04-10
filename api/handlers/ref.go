package handlers

import (
	"github.com/autarch/metagodoc/api/models"
	"github.com/autarch/metagodoc/api/restapi/operations"
	"github.com/autarch/metagodoc/esmodels"

	"github.com/go-openapi/runtime/middleware"
)

func (h *handlers) GetRepositoryRef(params operations.GetRepositoryRepositoryRefRefParams) middleware.Responder {
	esr, ref, status := h.getRef(params.Repository, params.Ref)
	if status != 0 {
		return operations.NewGetRepositoryRepositoryRefRefDefault(status)
	}

	return h.maybeRefOkResponse(ref)
}

func (h *handlers) maybeRefOkResponse(ref *esmodels.Ref) middleware.Responder {
	lsc, err := h.dt(ref.LastSeenCommit)
	if err != nil {
		return operations.NewGetRepositoryRepositoryRefRefDefault(500)
	}

	lu, err := h.dt(ref.LastUpdated)
	if err != nil {
		return operations.NewGetRepositoryRepositoryRefRefDefault(500)
	}

	return operations.NewGetRepositoryRepositoryRefRefOK().WithPayload(
		&models.Ref{
			IsDefaultBranch: ref.IsDefaultBranch,
			LastSeenCommit:  *lsc,
			LastUpdated:     *lu,
			Name:            ref.Name,
			Packages:        packages(ref.Packages),
			RefType:         ref.RefType,
		},
	)
}
