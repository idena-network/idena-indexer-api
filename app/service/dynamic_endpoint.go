package service

import (
	"github.com/idena-network/idena-indexer-api/app/db"
	"github.com/idena-network/idena-indexer-api/app/types"
)

type DynamicEndpointLoader interface {
	Load() ([]types.DynamicEndpoint, error)
}

type loaderImpl struct {
	dbAccessor db.Accessor
}

func NewDynamicEndpointLoader(dbAccessor db.Accessor) DynamicEndpointLoader {
	loader := &loaderImpl{
		dbAccessor: dbAccessor,
	}
	return loader
}

func (loader *loaderImpl) Load() ([]types.DynamicEndpoint, error) {
	return loader.dbAccessor.DynamicEndpoints()
}
