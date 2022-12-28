package postgres

import (
	"github.com/idena-network/idena-indexer-api/app/types"
	"strconv"
)

const (
	epochDelegateeRewardsQuery             = "epochDelegateeRewards.sql"
	epochAddressDelegateeTotalRewardsQuery = "epochAddressDelegateeTotalRewards.sql"
)

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
			&item.PrevState,
			&item.State,
			&reward,
			&rewards.validation,
			&rewards.flips,
			&rewards.extraFlips,
			&rewards.inv,
			&rewards.inv2,
			&rewards.inv3,
			&rewards.invitee,
			&rewards.invitee2,
			&rewards.invitee3,
			&rewards.savedInv,
			&rewards.savedInvWin,
			&rewards.reports,
			&rewards.candidate,
			&rewards.staking,
		)
		item.Rewards = toDelegationReward(rewards, a.replaceValidationReward)
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

func (a *postgresAccessor) EpochAddressDelegateeTotalRewards(epoch uint64, address string) (types.DelegateeTotalRewards, error) {
	rows, err := a.db.Query(a.getQuery(epochAddressDelegateeTotalRewardsQuery), epoch, address)
	if err != nil {
		return types.DelegateeTotalRewards{}, err
	}
	defer rows.Close()
	var res types.DelegateeTotalRewards
	if rows.Next() {
		var rewards validationRewards
		err = rows.Scan(
			&res.Epoch,
			&rewards.validation,
			&rewards.flips,
			&rewards.extraFlips,
			&rewards.inv,
			&rewards.inv2,
			&rewards.inv3,
			&rewards.invitee,
			&rewards.invitee2,
			&rewards.invitee3,
			&rewards.savedInv,
			&rewards.savedInvWin,
			&rewards.reports,
			&rewards.candidate,
			&rewards.staking,
			&res.Delegators,
			&res.PenalizedDelegators,
		)
		res.Rewards = toDelegationReward(rewards, a.replaceValidationReward)
		if err != nil {
			return types.DelegateeTotalRewards{}, err
		}
	}
	return res, nil
}
