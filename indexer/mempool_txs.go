package indexer

import (
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-indexer-api/app/types"
)

type MemPool struct {
	api Api
}

func NewMemPool(api Api) *MemPool {
	res := &MemPool{
		api: api,
	}
	return res
}

func (memPool *MemPool) GetTransaction(hash string) (*types.TransactionDetail, error) {
	return memPool.api.MemPoolTransaction(hash)
}

func (memPool *MemPool) GetTransactionRaw(hash string) (*hexutil.Bytes, error) {
	return memPool.api.MemPoolTransactionRaw(hash)
}

func (memPool *MemPool) GetAddressTransactions(address string, count int) ([]*types.TransactionSummary, error) {
	return memPool.api.MemPoolAddressTransactions(address, count)
}

func (memPool *MemPool) GetTransactions(count int) ([]*types.TransactionSummary, error) {
	return memPool.api.MemPoolTransactions(count)
}
