package elc

import (
	"github.com/autarch/metagodoc/logger"

	"github.com/olivere/elastic"
)

func NewClient(trace bool, l *logger.Logger) (*elastic.Client, error) {
	funcs := []elastic.ClientOptionFunc{}
	if l != nil {
		funcs = append(funcs, elastic.SetTraceLog(l))
	}

	return elastic.NewClient(funcs...)
}
