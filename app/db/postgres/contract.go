package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
)

const (
	contractQuery                 = "contract.sql"
	timeLockContractQuery         = "timeLockContract.sql"
	multisigContractQuery         = "multisigContract.sql"
	oracleLockContractQuery       = "oracleLockContract.sql"
	oracleVotingContractQuery     = "oracleVotingContract.sql"
	contractTxBalanceUpdatesQuery = "contractTxBalanceUpdates.sql"
)

func (a *postgresAccessor) Contract(address string) (types.Contract, error) {
	res := types.Contract{}
	var terminationTxTime sql.NullInt64
	var terminationTxHash sql.NullString
	var deployTxTimestamp int64
	err := a.db.QueryRow(a.getQuery(contractQuery), address).Scan(
		&res.Type,
		&res.Author,
		&res.DeployTx.Hash,
		&deployTxTimestamp,
		&terminationTxHash,
		&terminationTxTime,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.Contract{}, err
	}
	res.Address = address
	res.DeployTx.Timestamp = timestampToTimeUTCp(deployTxTimestamp)
	if terminationTxHash.Valid {
		res.TerminationTx = &types.TransactionSummary{
			Hash:      terminationTxHash.String,
			Timestamp: timestampToTimeUTCp(terminationTxTime.Int64),
		}
	}
	return res, nil
}

func (a *postgresAccessor) ContractTxBalanceUpdates(contractAddress string, count uint64, continuationToken *string) ([]types.ContractTxBalanceUpdate, *string, error) {
	res, nextContinuationToken, err := a.page(contractTxBalanceUpdatesQuery, func(rows *sql.Rows) (interface{}, uint64, error) {
		defer rows.Close()
		var res []types.ContractTxBalanceUpdate
		var id uint64
		for rows.Next() {
			item := types.ContractTxBalanceUpdate{}
			var timestamp int64
			var callMethod sql.NullInt32
			var balanceOld, balanceNew, gasCost NullDecimal
			var success sql.NullBool
			var gasUsed sql.NullInt64
			var method, errorMsg sql.NullString
			if err := rows.Scan(
				&id,
				&item.Hash,
				&item.Type,
				&timestamp,
				&item.From,
				&item.To,
				&item.Amount,
				&item.Tips,
				&item.MaxFee,
				&item.Fee,
				&item.Address,
				&item.ContractAddress,
				&item.ContractType,
				&callMethod,
				&balanceOld,
				&balanceNew,
				&success,
				&gasUsed,
				&gasCost,
				&method,
				&errorMsg,
			); err != nil {
				return nil, 0, err
			}
			item.Timestamp = timestampToTimeUTC(timestamp)
			if callMethod.Valid {
				item.ContractCallMethod = types.GetCallMethodName(item.ContractType, uint8(callMethod.Int32))
			}
			if balanceOld.Valid && balanceNew.Valid {
				change := balanceNew.Decimal.Sub(balanceOld.Decimal)
				item.BalanceChange = &change
			}
			if success.Valid {
				item.TxReceipt = &types.TxReceipt{
					Success:  success.Bool,
					GasUsed:  uint64(gasUsed.Int64),
					GasCost:  gasCost.Decimal,
					Method:   method.String,
					ErrorMsg: errorMsg.String,
				}
			}
			res = append(res, item)
		}
		return res, id, nil
	}, count, continuationToken, contractAddress)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.ContractTxBalanceUpdate), nextContinuationToken, nil
}

func (a *postgresAccessor) TimeLockContract(address string) (types.TimeLockContract, error) {
	res := types.TimeLockContract{}
	var timestamp int64
	err := a.db.QueryRow(a.getQuery(timeLockContractQuery), address).Scan(&timestamp)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.TimeLockContract{}, err
	}
	res.Timestamp = types.JSONTime(timestampToTimeUTC(timestamp))
	return res, nil
}

func (a *postgresAccessor) OracleLockContract(address string) (types.OracleLockContract, error) {
	res := types.OracleLockContract{}
	err := a.db.QueryRow(a.getQuery(oracleLockContractQuery), address).Scan(
		&res.OracleVotingAddress,
		&res.Value,
		&res.SuccessAddress,
		&res.FailAddress,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.OracleLockContract{}, err
	}
	return res, nil
}

func (a *postgresAccessor) MultisigContract(address string) (types.MultisigContract, error) {
	res := types.MultisigContract{}
	err := a.db.QueryRow(a.getQuery(multisigContractQuery), address).Scan(&res.MinVotes, &res.MaxVotes)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.MultisigContract{}, err
	}
	return res, nil
}

func (a *postgresAccessor) OracleVotingContract(address, oracle string) (types.OracleVotingContract, error) {
	rows, err := a.db.Query(a.getQuery(oracleVotingContractQuery), address, oracle)
	if err != nil {
		return types.OracleVotingContract{}, err
	}
	defer rows.Close()
	contracts, _, err := a.readOracleVotingContracts(rows)
	if err != nil {
		return types.OracleVotingContract{}, err
	}
	if len(contracts) == 0 {
		return types.OracleVotingContract{}, NoDataFound
	}
	return contracts[0], nil
}
