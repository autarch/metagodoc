package handlers

import (
	"github.com/autarch/metagodoc/api/restapi/operations"

	"github.com/go-openapi/runtime/middleware"
)

func (h *handlers) GetRepositoryRefPackage(
	params operations.GetRepositoryRepositoryRefRefPackagePackageParams,
) middleware.Responder {
	esr, ref, status := h.getRef(params.Repository, params.Ref, params.Package)
	if status != 0 {
		return operations.NewGetRepositoryRepositoryRefRefPackagePackageDefault(status)
	}

	return operations.NewGetRepositoryRepositoryRefRefPackagePackageOK().WithPayload(onePackage(pkg))
}
