package postgres

import (
	"database/sql"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	activationTx   = "ActivationTx"
	killTx         = "KillTx"
	killInviteeTx  = "KillInviteeTx"
	onlineStatusTx = "OnlineStatusTx"
)

func timestampToTimeUTC(timestamp int64) time.Time {
	return common.TimestampToTime(big.NewInt(timestamp)).UTC()
}

func timestampToTimeUTCp(timestamp int64) *time.Time {
	if timestamp == 0 {
		return nil
	}
	res := common.TimestampToTime(big.NewInt(timestamp)).UTC()
	return &res
}

type NullDecimal struct {
	Decimal decimal.Decimal
	Valid   bool
}

func (n *NullDecimal) Scan(value interface{}) error {
	n.Valid = value != nil
	n.Decimal = decimal.Decimal{}
	if n.Valid {
		return n.Decimal.Scan(value)
	}
	return nil
}

func (a *postgresAccessor) count(queryName string, args ...interface{}) (uint64, error) {
	var res uint64
	err := a.db.QueryRow(a.getQuery(queryName), args...).Scan(&res)
	if err != nil {
		return 0, err
	}
	return res, nil
}

func readInvites(rows *sql.Rows) ([]types.Invite, uint64, error) {
	defer rows.Close()
	var res []types.Invite
	var id uint64
	for rows.Next() {
		item := types.Invite{}
		var timestamp, activationTimestamp, killInviteeTimestamp int64
		if err := rows.Scan(
			&id,
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
		); err != nil {
			return nil, 0, err
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
	return res, id, nil
}

func readTxs(rows *sql.Rows) ([]types.TransactionSummary, uint64, error) {
	defer rows.Close()
	var res []types.TransactionSummary
	var id uint64
	for rows.Next() {
		item := types.TransactionSummary{}
		var timestamp int64
		var gasCost, transfer NullDecimal
		var success, becomeOnline sql.NullBool
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
			&item.Size,
			&item.Nonce,
			&transfer,
			&becomeOnline,
			&success,
			&gasUsed,
			&gasCost,
			&method,
			&errorMsg,
		); err != nil {
			return nil, 0, err
		}
		item.Timestamp = timestampToTimeUTCp(timestamp)
		if transfer.Valid {
			item.Transfer = &transfer.Decimal
		}
		item.Data = readTxSpecificData(item.Type, transfer, becomeOnline)
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
}

func readTxSpecificData(txType string, transfer NullDecimal, becomeOnline sql.NullBool) interface{} {
	var res interface{}
	switch txType {
	case activationTx:
		if data := readActivationTxSpecificData(transfer); data != nil {
			res = data
		}
	case killTx:
		if data := readKillTxSpecificData(transfer); data != nil {
			res = data
		}
	case killInviteeTx:
		if data := readKillInviteeTxSpecificData(transfer); data != nil {
			res = data
		}
	case onlineStatusTx:
		if data := readOnlineStatusTxSpecificData(becomeOnline); data != nil {
			res = data
		}
	}
	return res
}

func readActivationTxSpecificData(transfer NullDecimal) *types.ActivationTxSpecificData {
	if !transfer.Valid {
		return nil
	}
	s := transfer.Decimal.String()
	return &types.ActivationTxSpecificData{
		Transfer: &s,
	}
}

func readKillTxSpecificData(transfer NullDecimal) *types.KillTxSpecificData {
	return readActivationTxSpecificData(transfer)
}

func readKillInviteeTxSpecificData(transfer NullDecimal) *types.KillInviteeTxSpecificData {
	return readActivationTxSpecificData(transfer)
}

func readOnlineStatusTxSpecificData(becomeOnline sql.NullBool) *types.OnlineStatusTxSpecificData {
	if !becomeOnline.Valid {
		return nil
	}
	return &types.OnlineStatusTxSpecificData{
		BecomeOnline:    becomeOnline.Bool,
		BecomeOnlineOld: becomeOnline.Bool,
	}
}

func (a *postgresAccessor) strValueCounts(queryName string, args ...interface{}) ([]types.StrValueCount, error) {
	rows, err := a.db.Query(a.getQuery(queryName), args...)
	if err != nil {
		return nil, err
	}
	return readStrValueCounts(rows)
}

func readStrValueCounts(rows *sql.Rows) ([]types.StrValueCount, error) {
	defer rows.Close()
	var res []types.StrValueCount
	for rows.Next() {
		item := types.StrValueCount{}
		if err := rows.Scan(&item.Value, &item.Count); err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) flips(
	queryName string,
	count uint64,
	continuationToken *string,
	args ...interface{},
) ([]types.FlipSummary, *string, error) {
	res, nextContinuationToken, err := a.page(queryName, func(rows *sql.Rows) (interface{}, uint64, error) {
		return readFlips(rows)
	}, count, continuationToken, args...)
	if err != nil {
		return nil, nil, err
	}
	return res.([]types.FlipSummary), nextContinuationToken, nil
}

func readFlips(rows *sql.Rows) ([]types.FlipSummary, uint64, error) {
	defer rows.Close()
	var res []types.FlipSummary
	var id uint64
	for rows.Next() {
		item := types.FlipSummary{}
		var timestamp int64
		words := types.FlipWords{}
		err := rows.Scan(
			&id,
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
			&item.GradeScore,
		)
		if err != nil {
			return nil, 0, err
		}
		item.Timestamp = timestampToTimeUTC(timestamp)
		if !words.IsEmpty() {
			item.Words = &words
		}
		res = append(res, item)
	}
	return res, id, nil
}

func (a *postgresAccessor) flipsWithoutPaging(queryName string, args ...interface{}) ([]types.FlipSummary, error) {
	rows, err := a.db.Query(a.getQuery(queryName), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.FlipSummary
	for rows.Next() {
		item := types.FlipSummary{}
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
			&item.GradeScore,
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

func readEpochIdentities(rows *sql.Rows) ([]types.EpochIdentity, uint64, error) {
	defer rows.Close()
	var res []types.EpochIdentity
	var id uint64
	for rows.Next() {
		var item types.EpochIdentity
		var err error
		if item, id, err = readEpochIdentity(rows); err != nil {
			return nil, 0, err
		}
		res = append(res, item)
	}
	return res, id, nil
}

func readEpochIdentity(rows *sql.Rows) (types.EpochIdentity, uint64, error) {
	res := types.EpochIdentity{}
	var id uint64
	err := rows.Scan(
		&id,
		&res.Address,
		&res.Epoch,
		&res.State,
		&res.PrevState,
		&res.Approved,
		&res.Missed,
		&res.ShortAnswers.Point,
		&res.ShortAnswers.FlipsCount,
		&res.TotalShortAnswers.Point,
		&res.TotalShortAnswers.FlipsCount,
		&res.LongAnswers.Point,
		&res.LongAnswers.FlipsCount,
		&res.RequiredFlips,
		&res.MadeFlips,
		&res.AvailableFlips,
		&res.TotalValidationReward,
		&res.BirthEpoch,
		&res.ShortAnswersCount,
		&res.LongAnswersCount,
	)
	return res, id, err
}

func readAnswers(rows *sql.Rows) ([]types.Answer, error) {
	defer rows.Close()
	var res []types.Answer
	for rows.Next() {
		item := types.Answer{}
		err := rows.Scan(
			&item.Cid,
			&item.Address,
			&item.RespAnswer,
			&item.RespWrongWords,
			&item.FlipAnswer,
			&item.FlipWrongWords,
			&item.FlipStatus,
			&item.Point,
			&item.RespGrade,
			&item.FlipGrade,
			&item.Index,
			&item.Considered,
			&item.GradeIgnored,
		)
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, nil
}

func (a *postgresAccessor) coins(queryName string, args ...interface{}) (types.AllCoins, error) {
	res := types.AllCoins{}
	err := a.db.QueryRow(a.getQuery(queryName), args...).
		Scan(&res.Burnt,
			&res.Minted,
			&res.TotalBalance,
			&res.TotalStake)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.AllCoins{}, err
	}
	return res, nil
}

func (a *postgresAccessor) rewards(queryName string, args ...interface{}) ([]types.Reward, error) {
	rows, err := a.db.Query(a.getQuery(queryName), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []types.Reward
	for rows.Next() {
		item := types.Reward{}
		if err := rows.Scan(&item.Address, &item.Epoch, &item.BlockHeight, &item.Balance, &item.Stake, &item.Type); err != nil {
			return nil, err
		}
		if a.replaceValidationReward {
			item.Type = replaceCandidatesAndStaking(item.Type)
		}
		res = append(res, item)
	}
	return res, nil
}

func readBadAuthors(rows *sql.Rows) ([]types.BadAuthor, uint64, error) {
	defer rows.Close()
	var res []types.BadAuthor
	var id uint64
	for rows.Next() {
		item := types.BadAuthor{}
		err := rows.Scan(
			&id,
			&item.Address,
			&item.Epoch,
			&item.WrongWords,
			&item.Reason,
			&item.PrevState,
			&item.State,
		)
		if err != nil {
			return nil, 0, err
		}
		res = append(res, item)
	}
	return res, id, nil
}

func (a *postgresAccessor) adjacentStrValues(queryName string, value string) (types.AdjacentStrValues, error) {
	res := types.AdjacentStrValues{}
	err := a.db.QueryRow(a.getQuery(queryName), value).Scan(
		&res.Prev.Value,
		&res.Prev.Cycled,
		&res.Next.Value,
		&res.Next.Cycled,
	)
	if err == sql.ErrNoRows {
		err = NoDataFound
	}
	if err != nil {
		return types.AdjacentStrValues{}, err
	}
	return res, nil
}

func parseUintContinuationToken(continuationToken *string) (*uint64, error) {
	if continuationToken == nil {
		return nil, nil
	}
	var result *uint64
	var err error
	if num, parsingErr := strconv.ParseUint(*continuationToken, 10, 64); parsingErr != nil {
		err = errors.New("invalid continuation token")
	} else {
		result = &num
	}
	return result, err
}

func getResWithContinuationToken(lastToken string, count uint64, slice interface{}) (interface{}, *string) {
	var nextContinuationToken *string
	resSlice := reflect.ValueOf(slice)

	if resSlice.Len() > 0 && resSlice.Len() == int(count)+1 {
		nextContinuationToken = &lastToken
		resSlice = resSlice.Slice(0, resSlice.Len()-1)
	}
	return resSlice.Interface(), nextContinuationToken
}

func (a *postgresAccessor) page(
	queryName string,
	readRows func(rows *sql.Rows) (interface{}, uint64, error),
	count uint64,
	continuationToken *string,
	args ...interface{},
) (interface{}, *string, error) {
	var continuationId *uint64
	var err error
	if continuationId, err = parseUintContinuationToken(continuationToken); err != nil {
		return nil, nil, err
	}
	rows, err := a.db.Query(a.getQuery(queryName), append(args, count+1, continuationId)...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	res, nextId, err := readRows(rows)
	if err != nil {
		return nil, nil, err
	}
	resSlice, nextContinuationToken := getResWithContinuationToken(strconv.FormatUint(nextId, 10), count, res)
	return resSlice, nextContinuationToken, nil
}

func (a *postgresAccessor) page2(
	queryName string,
	readRows func(rows *sql.Rows) (interface{}, int64, error),
	count uint64,
	continuationToken *string,
	args ...interface{},
) (interface{}, *string, error) {
	var continuationId *int64
	var err error
	if continuationId, err = parseIntContinuationToken(continuationToken); err != nil {
		return nil, nil, err
	}
	rows, err := a.db.Query(a.getQuery(queryName), append(args, count+1, continuationId)...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	res, nextId, err := readRows(rows)
	if err != nil {
		return nil, nil, err
	}
	resSlice, nextContinuationToken := getResWithContinuationToken(strconv.FormatInt(nextId, 10), count, res)
	return resSlice, nextContinuationToken, nil
}

func parseIntContinuationToken(continuationToken *string) (*int64, error) {
	if continuationToken == nil {
		return nil, nil
	}
	var result *int64
	var err error
	if num, parsingErr := strconv.ParseInt(*continuationToken, 10, 64); parsingErr != nil {
		err = errors.New("invalid continuation token")
	} else {
		result = &num
	}
	return result, err
}

func parseUintAndAmountToken(continuationToken *string) (id *uint64, amount *decimal.Decimal, err error) {
	if continuationToken == nil {
		return
	}
	strs := strings.Split(*continuationToken, "-")
	if len(strs) != 2 {
		err = errors.New("invalid continuation token")
		return
	}
	sId := strs[0]
	if id, err = parseUintContinuationToken(&sId); err != nil {
		return
	}
	var d decimal.Decimal
	d, err = decimal.NewFromString(strs[1])
	if err != nil {
		return
	}
	amount = &d
	return
}

type validationRewards struct {
	validation, flips, extraFlips, inv, inv2, inv3, invitee, invitee2, invitee3, savedInv, savedInvWin, reports, candidate, staking decimal.Decimal
}

func toDelegationReward(vr validationRewards, replaceValidation bool) []types.DelegationReward {
	var res []types.DelegationReward
	if !vr.validation.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.validation,
			Type:    "Validation",
		})
	}
	if !vr.flips.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.flips,
			Type:    "Flips",
		})
	}
	if !vr.extraFlips.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.extraFlips,
			Type:    "ExtraFlips",
		})
	}
	if !vr.inv.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.inv,
			Type:    "Invitations",
		})
	}
	if !vr.inv2.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.inv2,
			Type:    "Invitations2",
		})
	}
	if !vr.inv3.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.inv3,
			Type:    "Invitations3",
		})
	}
	if !vr.invitee.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.invitee,
			Type:    "Invitee",
		})
	}
	if !vr.invitee2.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.invitee2,
			Type:    "Invitee2",
		})
	}
	if !vr.invitee3.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.invitee3,
			Type:    "Invitee3",
		})
	}
	if !vr.savedInv.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.savedInv,
			Type:    "SavedInvite",
		})
	}
	if !vr.savedInvWin.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.savedInvWin,
			Type:    "SavedInviteWin",
		})
	}
	if !vr.reports.IsZero() {
		res = append(res, types.DelegationReward{
			Balance: vr.reports,
			Type:    "Reports",
		})
	}
	if !vr.candidate.IsZero() {
		if replaceValidation {
			res = append(res, types.DelegationReward{
				Balance: vr.candidate,
				Type:    "Validation",
			})
		} else {
			res = append(res, types.DelegationReward{
				Balance: vr.candidate,
				Type:    "Candidate",
			})
		}
	}
	if !vr.staking.IsZero() {
		if replaceValidation {
			res = append(res, types.DelegationReward{
				Balance: vr.staking,
				Type:    "Validation",
			})
		} else {
			res = append(res, types.DelegationReward{
				Balance: vr.staking,
				Type:    "Staking",
			})
		}
	}
	return res
}

func replaceCandidatesAndStaking(rewardType string) string {
	if rewardType == "Candidate" || rewardType == "Staking" {
		return "Validation"
	}
	return rewardType
}

func feeRate(feeRate decimal.Decimal, height, embeddedContractForkHeight uint64) (feePerGas, feePerByte *decimal.Decimal) {
	if height < embeddedContractForkHeight {
		feePerByte = &feeRate
	} else {
		feePerGas = &feeRate
	}
	return
}
