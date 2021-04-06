package indexer

import (
	"github.com/idena-network/idena-indexer-api/app/service"
)

type ContractsMemPool struct {
	api Api
}

func NewContractsMemPool(api Api) *ContractsMemPool {
	res := &ContractsMemPool{
		api: api,
	}
	return res
}

func (memPool *ContractsMemPool) GetOracleVotingContractDeploys(author string) ([]service.OracleVotingContract, error) {
	return memPool.api.MemPoolOracleVotingContractDeploys(author)
}

func (memPool *ContractsMemPool) GetAddressContractTxs(address, contractAddress string) ([]service.Transaction, error) {
	return memPool.api.MemPoolAddressContractTxs(address, contractAddress)
}
