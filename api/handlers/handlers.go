package handlers

import (
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
