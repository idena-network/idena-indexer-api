package postgres

import (
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/shopspring/decimal"
	"sync"
	"time"
)

type estimatedOracleRewardsService struct {
	cache         *estimatedOracleRewardsServiceCache
	mutex         sync.Mutex
	networkSizeFn func() (uint64, error)
}

type estimatedOracleRewardsServiceCache struct {
	networkSize uint64
}

func newEstimatedOracleRewardsCache(
	networkSizeFn func() (uint64, error),
) *estimatedOracleRewardsService {
	res := &estimatedOracleRewardsService{
		networkSizeFn: networkSizeFn,
	}
	go func() {
		for {
			time.Sleep(time.Minute)
			res.cache = nil
		}
	}()
	return res
}

func (c *estimatedOracleRewardsService) get() ([]types.EstimatedOracleReward, error) {
	data := c.cache
	if data == nil {
		c.mutex.Lock()
		data = c.cache
		if data == nil {
			var err error
			data, err = c.loadData()
			if err != nil {
				c.mutex.Unlock()
				return nil, err
			}
		}
		c.mutex.Unlock()
	}
	return createEstimatedOracleRewardsService(data.networkSize), nil
}

func (c *estimatedOracleRewardsService) loadData() (*estimatedOracleRewardsServiceCache, error) {
	networkSize, err := c.networkSizeFn()
	if err != nil {
		return nil, err
	}
	return &estimatedOracleRewardsServiceCache{
		networkSize: networkSize,
	}, nil
}

func createEstimatedOracleRewardsService(networkSize uint64) []types.EstimatedOracleReward {
	if networkSize == 0 {
		networkSize = 1
	}
	minOracleReward := decimal.NewFromFloat(5000).Div(decimal.NewFromFloat(float64(networkSize)))
	return []types.EstimatedOracleReward{
		{
			Amount: minOracleReward,
			Type:   "min",
		},
		{
			Amount: decimal.NewFromFloat(2).Mul(minOracleReward),
			Type:   "low",
		},
		{
			Amount: decimal.NewFromFloat(4).Mul(minOracleReward),
			Type:   "medium",
		},
		{
			Amount: decimal.NewFromFloat(10).Mul(minOracleReward),
			Type:   "high",
		},
		{
			Amount: decimal.NewFromFloat(20).Mul(minOracleReward),
			Type:   "highest",
		},
	}
}
