package postgres

import (
	"github.com/idena-network/idena-indexer-api/app/types"
	"strconv"
)

const epochDelegateeRewardsQuery = "epochDelegateeRewards.sql"

func (a *postgresAccessor) EpochDelegateeRewards(epoch uint64, address string, count uint64, continuationToken *string) ([]types.DelegateeReward, *string, error) {
	addressId, reward, err := parseUintAndAmountToken(continuationToken)
	if err != nil {
		return nil, nil, err
	}
	rows, err := a.db.Query(a.getQuery(epochDelegateeRewardsQuery), epoch, address, count+1, reward, addressId)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var res []types.DelegateeReward
	for rows.Next() {
		item := types.DelegateeReward{}
		var rewards validationRewards
		err = rows.Scan(
			&addressId,
			&item.DelegatorAddress,
			&reward,
			&rewards.validation,
			&rewards.flips,
			&rewards.inv,
			&rewards.inv2,
			&rewards.inv3,
			&rewards.savedInv,
			&rewards.savedInvWin,
			&rewards.reports,
		)
		item.Rewards = toDelegationReward(rewards)
		if err != nil {
			return nil, nil, err
		}
		res = append(res, item)
	}
	var nextContinuationToken *string
	if len(res) > 0 && len(res) == int(count)+1 {
		t := strconv.FormatUint(*addressId, 10) + "-" + reward.String()
		nextContinuationToken = &t
		res = res[:len(res)-1]
	}
	return res, nextContinuationToken, nil
}
