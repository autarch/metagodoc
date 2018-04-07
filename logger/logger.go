package logger

import (
	"go.uber.org/zap"
)

const maxPrefix = 24

type NewParams struct {
	IsProd bool
}

type Logger struct {
	*zap.SugaredLogger
}

func New(p NewParams) (*Logger, error) {
	var l *zap.Logger
	var err error
	if p.IsProd {
		l, err = zap.NewProduction()
	} else {
		l, err = zap.NewDevelopment()
	}

	if err != nil {
		return nil, err
	}

	return &Logger{l.Sugar()}, nil
}

func (l *Logger) Printf(format string, v ...interface{}) {
	l.Infof(format, v...)
}
