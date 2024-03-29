package postgres

import (
	"database/sql"
	math2 "github.com/idena-network/idena-go/common/math"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"math"
	"strings"
	"time"
)

const (
	oracleVotingContractsAllQuery      = "oracleVotingContractsAll.sql"
	oracleVotingContractsAllOpenQuery  = "oracleVotingContractsAllOpen.sql"
	oracleVotingContractsByOracleQuery = "oracleVotingContractsByOracle.sql"
	ovcAllSortedByDtQuery              = "ovcAllSortedByDt.sql"
	ovcAllOpenSortedByDtQuery          = "ovcAllOpenSortedByDt.sql"
	ovcByOracleSortedByDtQuery         = "ovcByOracleSortedByDt.sql"
	addressOracleVotingContractsQuery  = "addressOracleVotingContracts.sql"
	lastBlockFeeRateQuery              = "lastBlockFeeRate.sql"

	oracleVotingStateOpen           = "open"
	oracleVotingStateVoted          = "voted"
	oracleVotingStateCounting       = "counting"
	oracleVotingStatePending        = "pending"
	oracleVotingStateArchive        = "archive"
	oracleVotingStateTerminated     = "terminated"
	oracleVotingStateCanBeProlonged = "canbeprolonged"
)

type contractsFilter struct {
	authorAddress       *string
	stateOpen           bool
	stateVoted          bool
	stateCounting       bool
	stateCanBeProlonged bool
	statePending        bool
	stateArchive        bool
	stateTerminated     bool
	sortByReward        bool
	all                 bool
}

func createContractsFilter(authorAddress string, states []string, all bool, sortBy, continuationToken *string) (*contractsFilter, error) {
	res := &contractsFilter{}
	for _, state := range states {
		switch strings.ToLower(state) {
		case oracleVotingStateOpen:
			res.stateOpen = true
		case oracleVotingStateVoted:
			res.stateVoted = true
		case oracleVotingStateCounting:
			res.stateCounting = true
		case oracleVotingStateCanBeProlonged:
			res.stateCanBeProlonged = true
		case oracleVotingStatePending:
			res.statePending = true
		case oracleVotingStateArchive:
			res.stateArchive = true
		case oracleVotingStateTerminated:
			res.stateTerminated = true
		default:
			return nil, errors.Errorf("unknown state %v", state)
		}
	}
	res.sortByReward = (res.stateOpen || res.statePending) && !(res.stateVoted || res.stateCounting || res.stateCanBeProlonged || res.stateArchive || res.stateTerminated)
	if sortBy != nil {
		if !res.sortByReward && *sortBy == "reward" {
			return nil, errors.New("invalid combination of values 'states[]' and 'sortBy'")
		}
		if res.sortByReward && *sortBy == "timestamp" {
			res.sortByReward = false
		}
	}
	if len(authorAddress) > 0 {
		res.authorAddress = &authorAddress
	}
	res.all = all
	if !res.sortByReward {
		if _, err := parseUintContinuationToken(continuationToken); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (a *postgresAccessor) OracleVotingContracts(authorAddress, oracleAddress string, states []string, all bool, sortBy *string, count uint64, continuationToken *string) ([]types.OracleVotingContract, *string, error) {
	filter, err := createContractsFilter(authorAddress, states, all, sortBy, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	var rows *sql.Rows
	if filter.all {
		if !filter.stateOpen && !filter.stateVoted {
			var queryName string
			if filter.sortByReward {
				queryName = oracleVotingContractsAllQuery
			} else {
				queryName = ovcAllSortedByDtQuery
			}
			rows, err = a.db.Query(a.getQuery(queryName), filter.authorAddress, oracleAddress,
				filter.statePending, filter.stateCounting, filter.stateArchive, filter.stateTerminated,
				filter.stateCanBeProlonged, count+1, continuationToken)
		} else {
			var queryName string
			if filter.sortByReward {
				queryName = oracleVotingContractsAllOpenQuery
			} else {
				queryName = ovcAllOpenSortedByDtQuery
			}
			rows, err = a.db.Query(a.getQuery(queryName), filter.authorAddress, oracleAddress,
				filter.statePending, filter.stateOpen, filter.stateVoted, filter.stateCounting, filter.stateArchive,
				filter.stateTerminated, filter.stateCanBeProlonged, count+1, continuationToken)
		}
	} else {
		var queryName string
		if filter.sortByReward {
			queryName = oracleVotingContractsByOracleQuery
		} else {
			queryName = ovcByOracleSortedByDtQuery
		}
		rows, err = a.db.Query(a.getQuery(queryName), filter.authorAddress, oracleAddress,
			filter.statePending, filter.stateOpen, filter.stateVoted, filter.stateCounting, filter.stateArchive,
			filter.stateTerminated, filter.stateCanBeProlonged, count+1, continuationToken)
	}

	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	res, lastContinuationToken, err := a.readOracleVotingContracts(rows)
	if err != nil {
		return nil, nil, err
	}

	var nextContinuationToken *string
	if len(res) > 0 && len(res) == int(count)+1 {
		nextContinuationToken = lastContinuationToken
		res = res[:len(res)-1]
	}
	return res, nextContinuationToken, nil
}

func (a *postgresAccessor) AddressOracleVotingContracts(address string, count uint64, continuationToken *string) ([]types.OracleVotingContract, *string, error) {
	rows, err := a.db.Query(a.getQuery(addressOracleVotingContractsQuery), address, count+1, continuationToken)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	res, lastContinuationToken, err := a.readOracleVotingContracts(rows)
	if err != nil {
		return nil, nil, err
	}
	var nextContinuationToken *string
	if len(res) > 0 && len(res) == int(count)+1 {
		nextContinuationToken = lastContinuationToken
		res = res[:len(res)-1]
	}
	return res, nextContinuationToken, nil
}

func (a *postgresAccessor) readOracleVotingContracts(rows *sql.Rows) ([]types.OracleVotingContract, *string, error) {
	var res []types.OracleVotingContract
	var lastContinuationToken string
	var curItem *types.OracleVotingContract
	var isFirst bool
	var networkSize *uint64
	for rows.Next() {
		item := types.OracleVotingContract{}
		var option, optionVotes, optionAllVotes, countingBlock, committeeEpoch sql.NullInt64
		var createTime, startTime, headBlockTimestamp int64
		var votingFinishTime, publicVotingFinishTime, finishTime, terminationTime sql.NullInt64
		var minPayment, totalReward, ownerDeposit, oracleRewardFund NullDecimal
		var headBlockHeight uint64
		if err := rows.Scan(
			&lastContinuationToken,
			&item.ContractAddress,
			&item.Author,
			&item.Balance,
			&item.Fact,
			&item.VoteProofsCount,
			&item.SecretVotesCount,
			&item.VotesCount,
			&item.State,
			&option,
			&optionVotes,
			&optionAllVotes,
			&createTime,
			&startTime,
			&headBlockHeight,
			&headBlockTimestamp,
			&votingFinishTime,
			&publicVotingFinishTime,
			&countingBlock,
			&minPayment,
			&item.Quorum,
			&item.CommitteeSize,
			&item.VotingDuration,
			&item.PublicVotingDuration,
			&item.WinnerThreshold,
			&item.OwnerFee,
			&item.IsOracle,
			&committeeEpoch,
			&finishTime,
			&terminationTime,
			&totalReward,
			&item.Stake,
			&item.EpochWithoutGrowth,
			&ownerDeposit,
			&oracleRewardFund,
			&item.RefundRecipient,
			&item.Hash,
		); err != nil {
			return nil, nil, err
		}
		if curItem == nil || curItem.ContractAddress != item.ContractAddress {
			if curItem != nil {
				res = append(res, *curItem)
			}
			curItem = &item
			isFirst = true
		}
		if option.Valid {
			curItem.Votes = append(curItem.Votes, types.OracleVotingContractOptionVotes{
				Option:   byte(option.Int64),
				Count:    uint64(optionVotes.Int64),
				AllCount: uint64(optionAllVotes.Int64),
			})
		}
		if !isFirst {
			continue
		}
		item.CreateTime = timestampToTimeUTC(createTime)
		item.StartTime = timestampToTimeUTC(startTime)
		itemState := strings.ToLower(item.State)
		if countingBlock.Valid {
			if itemState == oracleVotingStateOpen || itemState == oracleVotingStateVoted || itemState == oracleVotingStateCounting || itemState == oracleVotingStateCanBeProlonged || itemState == oracleVotingStateArchive {
				headBlockTime := timestampToTimeUTC(headBlockTimestamp)

				if networkSize == nil {
					size, err := a.networkSizeLoader.Load()
					if err != nil {
						return nil, nil, errors.Wrap(err, "Unable to load network size")
					}
					networkSize = &size
				}
				d, _ := item.Stake.Mul(decimal.NewFromInt(int64(*networkSize))).Div(decimal.NewFromInt(100)).Float64()
				terminationDays := uint64(math.Round(math.Pow(d, 1.0/3)))
				const blocksInDay = 4320

				estimatedTerminationTime := headBlockTime.Add(time.Second * 20 * time.Duration(uint64(countingBlock.Int64)-headBlockHeight+curItem.PublicVotingDuration+terminationDays*blocksInDay))
				item.EstimatedTerminationTime = &estimatedTerminationTime
				if itemState == oracleVotingStateOpen || itemState == oracleVotingStateVoted || itemState == oracleVotingStateCounting || itemState == oracleVotingStateCanBeProlonged {
					estimatedPublicVotingFinishTime := headBlockTime.Add(time.Second * 20 * time.Duration(uint64(countingBlock.Int64)-headBlockHeight+curItem.PublicVotingDuration))
					item.EstimatedPublicVotingFinishTime = &estimatedPublicVotingFinishTime
					if itemState == oracleVotingStateOpen || itemState == oracleVotingStateVoted {
						estimatedVotingFinishTime := headBlockTime.Add(time.Second * 20 * time.Duration(uint64(countingBlock.Int64)-headBlockHeight))
						item.EstimatedVotingFinishTime = &estimatedVotingFinishTime
					}
				}
			}
		}
		if itemState == oracleVotingStatePending && item.EstimatedTerminationTime == nil {
			v := item.StartTime.Add(time.Hour * 24 * 30)
			item.EstimatedTerminationTime = &v
		}
		if committeeEpoch.Valid {
			v := uint64(committeeEpoch.Int64)
			item.CommitteeEpoch = &v
		}
		if minPayment.Valid {
			item.MinPayment = &minPayment.Decimal
		}
		if votingFinishTime.Valid {
			v := timestampToTimeUTC(votingFinishTime.Int64)
			item.VotingFinishTime = &v
		}
		if publicVotingFinishTime.Valid {
			v := timestampToTimeUTC(publicVotingFinishTime.Int64)
			item.PublicVotingFinishTime = &v
		}
		if finishTime.Valid {
			v := timestampToTimeUTC(finishTime.Int64)
			item.FinishTime = &v
		}
		if terminationTime.Valid {
			v := timestampToTimeUTC(terminationTime.Int64)
			item.TerminationTime = &v
		}
		if totalReward.Valid {
			item.TotalReward = &totalReward.Decimal
		}
		if ownerDeposit.Valid {
			item.OwnerDeposit = &ownerDeposit.Decimal
		}
		if oracleRewardFund.Valid {
			item.OracleRewardFund = &oracleRewardFund.Decimal
		}

		if itemState == oracleVotingStatePending || itemState == oracleVotingStateOpen || itemState == oracleVotingStateVoted || itemState == oracleVotingStateCounting || itemState == oracleVotingStateCanBeProlonged {
			item.EstimatedOracleReward = calculateEstimatedOracleReward(item.Balance, item.MinPayment, item.OwnerFee, item.CommitteeSize, item.VoteProofsCount, item.OwnerDeposit, item.OracleRewardFund)
			item.EstimatedMaxOracleReward = calculateEstimatedMaxOracleReward(item.Balance, item.MinPayment, item.OwnerFee, item.CommitteeSize, item.Quorum, item.WinnerThreshold, item.VoteProofsCount, item.OwnerDeposit, item.OracleRewardFund)
			item.EstimatedTotalReward = calculateEstimatedTotalReward(item.Balance, item.MinPayment, item.OwnerFee, item.VoteProofsCount, item.OwnerDeposit, item.OracleRewardFund)
		}

		isFirst = false
	}
	if curItem != nil {
		res = append(res, *curItem)
	}
	return res, &lastContinuationToken, nil
}

func calculateEstimatedOwnerReward(
	balance decimal.Decimal,
	ownerDeposit *decimal.Decimal,
	oracleRewardFund *decimal.Decimal,
	votingMinPayment decimal.Decimal,
	committeeSize uint64,
	ownerFee uint8,
	ceil bool,
) decimal.Decimal {
	calculateUserLocks := func() decimal.Decimal {
		committeeSizeD := decimal.NewFromInt(int64(committeeSize))
		return votingMinPayment.Mul(committeeSizeD)
	}

	var ownerReward decimal.Decimal
	if ownerDeposit != nil {
		ownerReward = ownerReward.Add(*ownerDeposit)
		if ownerFee > 0 {
			replenishedAmount := balance.Sub(*ownerDeposit)
			if oracleRewardFund != nil {
				replenishedAmount = replenishedAmount.Sub(*oracleRewardFund)
			}
			replenishedAmount = replenishedAmount.Sub(calculateUserLocks())
			if replenishedAmount.Sign() > 0 {
				ownerFeeD := decimal.NewFromFloat(float64(ownerFee) / 100.0)
				if ceil {
					ownerFeeD = ownerFeeD.Ceil()
				}
				feeAmount := replenishedAmount.Mul(ownerFeeD)
				ownerReward = ownerReward.Add(feeAmount)
			}
		}
	} else {
		if ownerFee > 0 {
			ownerFeeD := decimal.NewFromFloat(float64(ownerFee) / 100.0)
			if ceil {
				ownerFeeD = ownerFeeD.Ceil()
			}
			ownerReward = balance.Sub(calculateUserLocks()).Mul(ownerFeeD)
		}
	}
	return ownerReward
}

func calculateEstimatedOracleReward(
	balance decimal.Decimal,
	votingMinPaymentP *decimal.Decimal,
	ownerFee uint8,
	committeeSize,
	votesCnt uint64,
	ownerDeposit *decimal.Decimal,
	oracleRewardFund *decimal.Decimal,
) *decimal.Decimal {
	var votingMinPayment decimal.Decimal
	if votingMinPaymentP != nil {
		votingMinPayment = *votingMinPaymentP
	}
	potentialBalance := balance
	if committeeSize == 0 {
		committeeSize = 1
	}
	if votesCnt > committeeSize {
		committeeSize = votesCnt
	}
	if committeeSize > votesCnt && votingMinPayment.Sign() == 1 {
		potentialBalance = potentialBalance.Add(votingMinPayment.Mul(decimal.NewFromInt(int64(committeeSize - votesCnt))))
	}
	ownerReward := calculateEstimatedOwnerReward(potentialBalance, ownerDeposit, oracleRewardFund, votingMinPayment, committeeSize, ownerFee, true)
	oracleReward := potentialBalance.Sub(ownerReward).Div(decimal.NewFromInt(int64(committeeSize)))
	if oracleReward.Sign() < 0 {
		oracleReward = decimal.NewFromInt32(0)
	}
	return &oracleReward
}

func calculateEstimatedMaxOracleReward(
	balance decimal.Decimal,
	votingMinPaymentP *decimal.Decimal,
	ownerFee uint8,
	committeeSize uint64,
	quorum,
	winnerThreshold byte,
	votesCnt uint64,
	ownerDeposit *decimal.Decimal,
	oracleRewardFund *decimal.Decimal,
) *decimal.Decimal {
	quorumSizeF := float64(committeeSize) * float64(quorum) / 100.0
	quorumSize := uint64(quorumSizeF)
	if quorumSizeF > float64(quorumSize) || quorumSize == 0 {
		quorumSize += 1
	}
	minVotesCnt := math2.Max(quorumSize, votesCnt)

	var votingMinPayment decimal.Decimal
	if votingMinPaymentP != nil {
		votingMinPayment = *votingMinPaymentP
	}

	potentialBalance := balance
	if minVotesCnt > votesCnt && votingMinPayment.Sign() == 1 {
		potentialBalance = potentialBalance.Add(votingMinPayment.Mul(decimal.NewFromInt(int64(minVotesCnt - votesCnt))))
	}

	ownerReward := calculateEstimatedOwnerReward(potentialBalance, ownerDeposit, oracleRewardFund, votingMinPayment, minVotesCnt, ownerFee, false)

	oracleReward := potentialBalance.Sub(ownerReward).Div(decimal.NewFromInt(int64(minVotesCnt)).Mul(decimal.New(int64(winnerThreshold), -2)).Ceil())
	if oracleReward.Sign() < 0 {
		oracleReward = decimal.NewFromInt32(0)
	}
	return &oracleReward
}

func calculateEstimatedTotalReward(
	balance decimal.Decimal,
	votingMinPaymentP *decimal.Decimal,
	ownerFee uint8,
	votesCnt uint64,
	ownerDeposit *decimal.Decimal,
	oracleRewardFund *decimal.Decimal,
) *decimal.Decimal {
	var votingMinPayment decimal.Decimal
	if votingMinPaymentP != nil {
		votingMinPayment = *votingMinPaymentP
	}
	ownerReward := calculateEstimatedOwnerReward(balance, ownerDeposit, oracleRewardFund, votingMinPayment, votesCnt, ownerFee, false)
	totalReward := balance.Sub(ownerReward)
	if totalReward.Sign() < 0 {
		totalReward = decimal.NewFromInt32(0)
	}
	return &totalReward
}

func (a *postgresAccessor) EstimatedOracleRewards() ([]types.EstimatedOracleReward, error) {
	return a.estimatedOracleRewardsCache.get()
}

func (a *postgresAccessor) lastBlockFeeRate() (decimal.Decimal, error) {
	rows, err := a.db.Query(a.getQuery(lastBlockFeeRateQuery))
	var res decimal.Decimal
	if err != nil {
		return res, err
	}
	defer rows.Close()
	if rows.Next() {
		if err = rows.Scan(&res); err != nil {
			return res, err
		}
	}
	return res, nil
}
