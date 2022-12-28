package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/shopspring/decimal"
	"sort"
)

const (
	epochIdentityQuery                    = "epochIdentity.sql"
	epochIdentityAnswersQuery             = "epochIdentityAnswers.sql"
	epochIdentityFlipsToSolveQuery        = "epochIdentityFlipsToSolve.sql"
	epochIdentityFlipsQuery               = "epochIdentityFlips.sql"
	epochIdentityRewardedFlipsQuery       = "epochIdentityRewardedFlips.sql"
	epochIdentityReportedFlipRewardsQuery = "epochIdentityReportedFlipRewards.sql"
	epochIdentityRewardsQuery             = "epochIdentityRewards.sql"
	epochIdentityBadAuthorQuery           = "epochIdentityBadAuthor.sql"
	epochIdentityRewardedInvitesQuery     = "epochIdentityRewardedInvites.sql"
	epochIdentitySavedInviteRewardsQuery  = "epochIdentitySavedInviteRewards.sql"
	epochIdentityAvailableInvitesQuery    = "epochIdentityAvailableInvites.sql"
	epochIdentityValidationSummaryQuery   = "epochIdentityValidationSummary.sql"
)

func (a *postgresAccessor) EpochIdentity(epoch uint64, address string) (types.EpochIdentity, error) {
	res := types.EpochIdentity{}
	err := a.db.QueryRow(a.getQuery(epochIdentityQuery), epoch, address).Scan(
		&res.State,
		&res.PrevState,
		&res.ShortAnswers.Point,
		&res.ShortAnswers.FlipsCount,
		&res.TotalShortAnswers.Point,
		&res.TotalShortAnswers.FlipsCount,
		&res.LongAnswers.Point,
		&res.LongAnswers.FlipsCount,
		&res.Approved,
		&res.Missed,
		&res.RequiredFlips,
		&res.MadeFlips,
		&res.AvailableFlips,
		&res.TotalValidationReward,
		&res.BirthEpoch,
		&res.ShortAnswersCount,
		&res.LongAnswersCount,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.EpochIdentity{}, err
	}
	return res, nil
}

func (a *postgresAccessor) EpochIdentityShortFlipsToSolve(epoch uint64, address string) ([]string, error) {
	return a.epochIdentityFlipsToSolve(epoch, address, true)
}

func (a *postgresAccessor) EpochIdentityLongFlipsToSolve(epoch uint64, address string) ([]string, error) {
	return a.epochIdentityFlipsToSolve(epoch, address, false)
}

func (a *postgresAccessor) epochIdentityFlipsToSolve(epoch uint64, address string, isShort bool) ([]string, error) {
	rows, err := a.db.Query(a.getQuery(epochIdentityFlipsToSolveQuery), epoch, address, isShort)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var item string
		err = rows.Scan(&item)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) EpochIdentityShortAnswers(epoch uint64, address string) ([]types.Answer, error) {
	return a.epochIdentityAnswers(epoch, address, true)
}

func (a *postgresAccessor) EpochIdentityLongAnswers(epoch uint64, address string) ([]types.Answer, error) {
	return a.epochIdentityAnswers(epoch, address, false)
}

func (a *postgresAccessor) epochIdentityAnswers(epoch uint64, address string, isShort bool) ([]types.Answer, error) {
	rows, err := a.db.Query(a.getQuery(epochIdentityAnswersQuery), epoch, address, isShort)
	if err != nil {
		return nil, err
	}
	res, err := readAnswers(rows)
	if err != nil {
		return nil, err
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Index < res[j].Index
	})

	for isShort && len(res) > 0 && !res[len(res)-1].Considered && res[len(res)-1].RespAnswer == "None" {
		res = res[:len(res)-1]
	}
	return res, nil
}

func (a *postgresAccessor) EpochIdentityFlips(epoch uint64, address string) ([]types.FlipSummary, error) {
	return a.flipsWithoutPaging(epochIdentityFlipsQuery, epoch, address)
}

func (a *postgresAccessor) EpochIdentityFlipsWithRewardFlag(epoch uint64, address string) ([]types.FlipWithRewardFlag, error) {
	rows, err := a.db.Query(a.getQuery(epochIdentityRewardedFlipsQuery), epoch, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.FlipWithRewardFlag
	for rows.Next() {
		item := types.FlipWithRewardFlag{}
		var timestamp int64
		words := types.FlipWords{}
		err := rows.Scan(
			&item.Cid,
			&item.Size,
			&item.Author,
			&item.Epoch,
			&item.Status,
			&item.Answer,
			&item.WrongWords,
			&item.WrongWordsVotes,
			&item.ShortRespCount,
			&item.LongRespCount,
			&timestamp,
			&item.Icon,
			&words.Word1.Index,
			&words.Word1.Name,
			&words.Word1.Desc,
			&words.Word2.Index,
			&words.Word2.Name,
			&words.Word2.Desc,
			&item.WithPrivatePart,
			&item.Grade,
			&item.Rewarded,
		)
		if err != nil {
			return nil, err
		}
		item.Timestamp = timestampToTimeUTC(timestamp)
		if !words.IsEmpty() {
			item.Words = &words
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) EpochIdentityReportedFlipRewards(epoch uint64, address string) ([]types.ReportedFlipReward, error) {
	rows, err := a.db.Query(a.getQuery(epochIdentityReportedFlipRewardsQuery), epoch, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.ReportedFlipReward
	for rows.Next() {
		item := types.ReportedFlipReward{}
		words := types.FlipWords{}
		err := rows.Scan(
			&item.Cid,
			&item.Icon,
			&item.Author,
			&words.Word1.Index,
			&words.Word1.Name,
			&words.Word1.Desc,
			&words.Word2.Index,
			&words.Word2.Name,
			&words.Word2.Desc,
			&item.Balance,
			&item.Stake,
			&item.Grade,
		)
		if err != nil {
			return nil, err
		}
		if !words.IsEmpty() {
			item.Words = &words
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) EpochIdentityRewards(epoch uint64, address string) ([]types.Reward, error) {
	return a.rewards(epochIdentityRewardsQuery, epoch, address)
}

func (a *postgresAccessor) EpochIdentityBadAuthor(epoch uint64, address string) (*types.BadAuthor, error) {
	res := types.BadAuthor{}
	err := a.db.QueryRow(a.getQuery(epochIdentityBadAuthorQuery), epoch, address).Scan(
		&res.Address,
		&res.Epoch,
		&res.WrongWords,
		&res.Reason,
		&res.PrevState,
		&res.State,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (a *postgresAccessor) EpochIdentityInvitesWithRewardFlag(epoch uint64, address string) ([]types.InviteWithRewardFlag, error) {
	rows, err := a.db.Query(a.getQuery(epochIdentityRewardedInvitesQuery), epoch, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.InviteWithRewardFlag
	for rows.Next() {
		item := types.InviteWithRewardFlag{}
		var timestamp, activationTimestamp, killInviteeTimestamp int64
		if err := rows.Scan(
			&item.Hash,
			&item.Author,
			&timestamp,
			&item.Epoch,
			&item.ActivationHash,
			&item.ActivationAuthor,
			&activationTimestamp,
			&item.State,
			&item.KillInviteeHash,
			&killInviteeTimestamp,
			&item.KillInviteeEpoch,
			&item.RewardType,
			&item.EpochHeight,
		); err != nil {
			return nil, err
		}
		item.Timestamp = timestampToTimeUTC(timestamp)
		if activationTimestamp > 0 {
			t := timestampToTimeUTC(activationTimestamp)
			item.ActivationTimestamp = &t
		}
		if killInviteeTimestamp > 0 {
			t := timestampToTimeUTC(killInviteeTimestamp)
			item.KillInviteeTimestamp = &t
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) EpochIdentitySavedInviteRewards(epoch uint64, address string) ([]types.StrValueCount, error) {
	return a.strValueCounts(epochIdentitySavedInviteRewardsQuery, epoch, address)
}

func (a *postgresAccessor) EpochIdentityAvailableInvites(epoch uint64, address string) ([]types.EpochInvites, error) {
	rows, err := a.db.Query(a.getQuery(epochIdentityAvailableInvitesQuery), epoch, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.EpochInvites
	for rows.Next() {
		item := types.EpochInvites{}
		if err := rows.Scan(&item.Epoch, &item.Invites); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) EpochIdentityInviteeWithRewardFlag(epoch uint64, address string) (*types.InviteeWithRewardFlag, error) {
	// todo
	return nil, nil
}

func (a *postgresAccessor) EpochIdentityValidationSummary(epoch uint64, address string) (types.ValidationSummary, error) {
	res := types.ValidationSummary{}
	var validationReason, flipsReason, reportsReason, invitationsReason, candidateReason, stakingReason byte
	var delegateeAddress string
	var delegateeReward decimal.Decimal
	err := a.db.QueryRow(a.getQuery(epochIdentityValidationSummaryQuery), epoch, address).Scan(
		&res.ShortAnswers.Point,
		&res.ShortAnswers.FlipsCount,
		&res.TotalShortAnswers.Point,
		&res.TotalShortAnswers.FlipsCount,
		&res.LongAnswers.Point,
		&res.LongAnswers.FlipsCount,
		&res.ShortAnswersCount,
		&res.LongAnswersCount,
		&res.PrevState,
		&res.State,
		&res.Penalized,
		&res.PenaltyReason,
		&res.Approved,
		&res.Missed,
		&res.MadeFlips,
		&res.AvailableFlips,
		&res.Rewards.Validation.Earned,
		&res.Rewards.Validation.Missed,
		&validationReason,
		&res.Rewards.Flips.Earned,
		&res.Rewards.Flips.Missed,
		&flipsReason,
		&res.Rewards.Invitations.Earned,
		&res.Rewards.Invitations.Missed,
		&invitationsReason,
		&res.Rewards.Reports.Earned,
		&res.Rewards.Reports.Missed,
		&reportsReason,
		&res.Rewards.Candidate.Earned,
		&res.Rewards.Candidate.Missed,
		&candidateReason,
		&res.Rewards.Staking.Earned,
		&res.Rewards.Staking.Missed,
		&stakingReason,
		&delegateeAddress,
		&delegateeReward,
		&res.Stake,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.ValidationSummary{}, err
	}
	res.Rewards.Validation.MissedReason = convertMissedRewardReason(validationReason)
	res.Rewards.Flips.MissedReason = convertMissedRewardReason(flipsReason)
	res.Rewards.Invitations.MissedReason = convertMissedRewardReason(invitationsReason)
	res.Rewards.Reports.MissedReason = convertMissedRewardReason(reportsReason)
	res.Rewards.Candidate.MissedReason = convertMissedRewardReason(candidateReason)
	res.Rewards.Staking.MissedReason = convertMissedRewardReason(stakingReason)

	if a.replaceValidationReward {
		if res.Rewards.Candidate.Earned.Sign() > 0 || res.Rewards.Candidate.Missed.Sign() > 0 || len(res.Rewards.Candidate.MissedReason) > 0 ||
			res.Rewards.Staking.Earned.Sign() > 0 || res.Rewards.Staking.Missed.Sign() > 0 || len(res.Rewards.Staking.MissedReason) > 0 {
			res.Rewards.Validation.Earned = res.Rewards.Candidate.Earned.Add(res.Rewards.Staking.Earned)
			res.Rewards.Validation.Missed = res.Rewards.Candidate.Missed.Add(res.Rewards.Staking.Missed)
			res.Rewards.Validation.MissedReason = res.Rewards.Candidate.MissedReason
			if len(res.Rewards.Validation.MissedReason) == 0 {
				res.Rewards.Validation.MissedReason = res.Rewards.Staking.MissedReason
			}
		}
	}

	if len(delegateeAddress) > 0 {
		res.DelegateeReward = &types.ValidationDelegateeReward{
			Address: delegateeAddress,
			Amount:  delegateeReward,
		}
	}
	return res, nil
}

func convertMissedRewardReason(code byte) string {
	switch code {
	case 0:
		return ""
	case 1:
		return "penalty"
	case 2:
		return "not_validated"
	case 3:
		return "missed"
	case 4:
		return "not_all_flips"
	case 5:
		return "not_all_reports"
	case 6:
		return "not_all_invites"
	default:
		return "unknown_reason"
	}
}
