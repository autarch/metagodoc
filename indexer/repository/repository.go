package repository

import "github.com/autarch/metagodoc/esmodels"

type Repository interface {
	ESModel() *esmodels.Repository
	ID() string
}
