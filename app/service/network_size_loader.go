package service

import (
	"github.com/patrickmn/go-cache"
	"sync"
	"time"
)

const (
	successResponseExpiration = time.Minute
	errResponseExpiration     = time.Second * 10
)

type NetworkSizeLoader interface {
	Load() (uint64, error)
}

func NewCachedNetworkSizeLoader(loader NetworkSizeLoader) NetworkSizeLoader {
	res := &cachedNetworkSizeLoader{
		cache:  cache.New(successResponseExpiration, successResponseExpiration*5),
		loader: loader,
	}
	return res
}

type cachedNetworkSizeLoader struct {
	cache  *cache.Cache
	mutex  sync.Mutex
	loader NetworkSizeLoader
}

type resWrapper struct {
	res uint64
	err error
}

func (l *cachedNetworkSizeLoader) Load() (uint64, error) {
	const name = "data"
	wrapper, ok := l.cache.Get(name)
	if ok {
		return wrapper.(*resWrapper).res, wrapper.(*resWrapper).err
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	wrapper, ok = l.cache.Get(name)
	if ok {
		return wrapper.(*resWrapper).res, wrapper.(*resWrapper).err
	}

	res, err := l.loader.Load()
	wrapper = &resWrapper{
		res: res,
		err: err,
	}
	var expiration time.Duration
	if err == nil {
		expiration = cache.DefaultExpiration
	} else {
		expiration = errResponseExpiration
	}
	l.cache.Set(name, wrapper, expiration)
	return wrapper.(*resWrapper).res, wrapper.(*resWrapper).err
}
