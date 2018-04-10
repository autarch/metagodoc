package handlers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/autarch/metagodoc/esmodels"
	"github.com/autarch/metagodoc/logger"

	"github.com/go-openapi/strfmt"
	"github.com/olivere/elastic"
)

type handlers struct {
	l  *logger.Logger
	el *elastic.Client
}

func New(l *logger.Logger, el *elastic.Client) *handlers {
	return &handlers{l, el}
}

func (h *handlers) getRepo(repo string) (*esmodels.Repository, int) {
	result, err := h.el.Get().
		Index("metagodoc-repository").
		Type("repository").
		Id(repo).
		Do(context.Background())
	if err != nil {
		h.l.Errorf("Elastic get failed: %s", err)
		return nil, 500
	}

	if !result.Found {
		return nil, 404
	}

	esr := &esmodels.Repository{}
	err = json.Unmarshal(*result.Source, esr)
	if err != nil {
		h.l.Errorf("Unmarshal: %s", err)
		return nil, 500
	}

	return esr, nil
}

func (h *handlers) getRef(repo, ref string) (*esmodels.Repository, *esmodels.Ref, int) {
	esr, status := h.getRepo(repo)
	if status != 0 {
		return nil, nil, status
	}

	var esref *esmodels.Ref
	for _, r := range esr.Refs {
		if r.Name == ref {
			esref = r
			break
		}
	}

	if ref == nil {
		return nil, nil, 404
	}

	return esr, esref, 0
}

func (h *handlers) getPackage(repo, ref, pkg string) (*esmodels.Repository, *esmodels.Ref, *esmodels.Package, int) {
	esr, esref, status := h.getRef(repo, ref)
	if status != 0 {
		return nil, nil, nil, status
	}

	var esp *esmodels.Package
	for _, p := range esref.Packages {
		if p.Name == pkg {
			esp = pkg
			break
		}
	}

	if esp == nil {
		return nil, nil, nil, 404
	}

	return esr, esref, esp, 0
}

func (h *handlers) dt(val string) (*strfmt.DateTime, error) {
	t, err := time.Parse(esmodels.DateTimeFormat, val)
	if err != nil {
		h.l.Errorf("Could not parse datetime %s: %s", val, err)
		// The actual time returned is irrelevant since err is not nil.
		return nil, err
	}

	dt := strfmt.DateTime(t)
	return &dt, nil
}
