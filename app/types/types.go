package types

import (
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/shopspring/decimal"
	"time"
)

type JSONTime time.Time

func (t JSONTime) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(time.RFC3339Nano)+2)
	b = append(b, '"')
	b = time.Time(t).AppendFormat(b, time.RFC3339Nano)
	b = append(b, '"')
	return b, nil
}

type Entity struct {
	NameOld  string `json:"Name" swaggerignore:"true"`  // todo deprecated
	ValueOld string `json:"Value" swaggerignore:"true"` // todo deprecated
	RefOld   string `json:"Ref" swaggerignore:"true"`   // todo deprecated
	Name     string `json:"name" enums:"Address,Identity,Epoch,Block,Transaction,Flip"`
	Value    string `json:"value"`
	Ref      string `json:"ref"`
} // @Name Entity

type EpochSummary struct {
	Epoch                        uint64           `json:"epoch"`
	ValidationTime               time.Time        `json:"validationTime" example:"2020-01-01T00:00:00Z"`
	ValidatedCount               uint32           `json:"validatedCount"`
	BlockCount                   uint32           `json:"blockCount"`
	EmptyBlockCount              uint32           `json:"emptyBlockCount"`
	TxCount                      uint32           `json:"txCount"`
	InviteCount                  uint32           `json:"inviteCount"`
	CandidateCount               uint32           `json:"candidateCount"`
	FlipCount                    uint32           `json:"flipCount"`
	Coins                        AllCoins         `json:"coins"`
	Rewards                      RewardsSummary   `json:"rewards"`
	MinScoreForInvite            float32          `json:"minScoreForInvite"`
	DiscriminationStakeThreshold *decimal.Decimal `json:"discriminationStakeThreshold,omitempty" swaggertype:"string"`
} // @Name EpochSummary

type AllCoins struct {
	Minted       decimal.Decimal `json:"minted" swaggertype:"string"`
	Burnt        decimal.Decimal `json:"burnt" swaggertype:"string"`
	TotalBalance decimal.Decimal `json:"totalBalance" swaggertype:"string"`
	TotalStake   decimal.Decimal `json:"totalStake" swaggertype:"string"`
} // @Name Coins

type RewardsSummary struct {
	Epoch              uint64          `json:"epoch,omitempty"`
	Total              decimal.Decimal `json:"total" swaggertype:"string"`
	Validation         decimal.Decimal `json:"validation" swaggertype:"string"`
	Flips              decimal.Decimal `json:"flips" swaggertype:"string"`
	ExtraFlips         decimal.Decimal `json:"extraFlips" swaggertype:"string"`
	Invitations        decimal.Decimal `json:"invitations" swaggertype:"string"`
	Reports            decimal.Decimal `json:"reports" swaggertype:"string"`
	Candidate          decimal.Decimal `json:"candidate" swaggertype:"string"`
	Staking            decimal.Decimal `json:"staking" swaggertype:"string"`
	FoundationPayouts  decimal.Decimal `json:"foundationPayouts" swaggertype:"string"`
	ZeroWalletFund     decimal.Decimal `json:"zeroWalletFund" swaggertype:"string"`
	ValidationShare    decimal.Decimal `json:"validationShare" swaggertype:"string"`
	FlipsShare         decimal.Decimal `json:"flipsShare" swaggertype:"string"`
	ExtraFlipsShare    decimal.Decimal `json:"extraFlipsShare" swaggertype:"string"`
	InvitationsShare   decimal.Decimal `json:"invitationsShare" swaggertype:"string"`
	ReportsShare       decimal.Decimal `json:"reportsShare" swaggertype:"string"`
	CandidateShare     decimal.Decimal `json:"candidateShare" swaggertype:"string"`
	StakingShare       decimal.Decimal `json:"stakingShare" swaggertype:"string"`
	EpochDuration      uint32          `json:"epochDuration"`
	PrevEpochDurations []uint32        `json:"prevEpochDurations"`
} // @Name RewardsSummary

type EpochDetail struct {
	Epoch                        uint64           `json:"epoch"`
	ValidationTime               time.Time        `json:"validationTime" example:"2020-01-01T00:00:00Z"`
	StateRoot                    *string          `json:"stateRoot,omitempty"`
	ValidationFirstBlockHeight   uint64           `json:"validationFirstBlockHeight"`
	MinScoreForInvite            float32          `json:"minScoreForInvite"`
	CandidateCount               uint32           `json:"candidateCount"`
	DiscriminationStakeThreshold *decimal.Decimal `json:"discriminationStakeThreshold,omitempty" swaggertype:"string"`
} // @Name Epoch

type BlockSummary struct {
	Height               uint64           `json:"height"`
	Hash                 string           `json:"hash"`
	Timestamp            time.Time        `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	TxCount              uint16           `json:"txCount"`
	IsEmpty              bool             `json:"isEmpty"`
	Coins                AllCoins         `json:"coins"`
	BodySize             uint32           `json:"bodySize"`
	FullSize             uint32           `json:"fullSize"`
	VrfProposerThreshold float64          `json:"vrfProposerThreshold"`
	Proposer             string           `json:"proposer"`
	ProposerVrfScore     float64          `json:"proposerVrfScore,omitempty"`
	FeeRate              *decimal.Decimal `json:"feeRate,omitempty" swaggertype:"string"`
	FeeRatePerByte       *decimal.Decimal `json:"feeRatePerByte,omitempty" swaggertype:"string"`
	Flags                []string         `json:"flags" enums:"IdentityUpdate,FlipLotteryStarted,ShortSessionStarted,LongSessionStarted,AfterLongSessionStarted,ValidationFinished,Snapshot,OfflinePropose,OfflineCommit,NewGenesis"`
	Upgrade              *uint32          `json:"upgrade,omitempty"`
	OfflineAddress       *string          `json:"offlineAddress,omitempty"`
	Epoch                uint64           `json:"epoch,omitempty"`
} // @Name BlockSummary

type BlockDetail struct {
	Epoch                uint64           `json:"epoch"`
	Height               uint64           `json:"height"`
	Hash                 string           `json:"hash"`
	Timestamp            time.Time        `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	TxCount              uint16           `json:"txCount"`
	ValidatorsCount      uint16           `json:"validatorsCount"`
	IsEmpty              bool             `json:"isEmpty"`
	BodySize             uint32           `json:"bodySize"`
	FullSize             uint32           `json:"fullSize"`
	VrfProposerThreshold float64          `json:"vrfProposerThreshold"`
	Proposer             string           `json:"proposer"`
	ProposerVrfScore     float64          `json:"proposerVrfScore,omitempty"`
	FeeRate              *decimal.Decimal `json:"feeRate,omitempty" swaggertype:"string"`
	FeeRatePerByte       *decimal.Decimal `json:"feeRatePerByte,omitempty" swaggertype:"string"`
	Flags                []string         `json:"flags" enums:"IdentityUpdate,FlipLotteryStarted,ShortSessionStarted,LongSessionStarted,AfterLongSessionStarted,ValidationFinished,Snapshot,OfflinePropose,OfflineCommit,NewGenesis"`
	Upgrade              *uint32          `json:"upgrade,omitempty"`
	OfflineAddress       *string          `json:"offlineAddress,omitempty"`
} // @Name Block

type FlipSummary struct {
	Cid            string `json:"cid"`
	Author         string `json:"author"`
	Epoch          uint64 `json:"epoch"`
	ShortRespCount uint32 `json:"shortRespCount"`
	LongRespCount  uint32 `json:"longRespCount"`
	Status         string `json:"status" enums:",NotQualified,Qualified,WeaklyQualified,QualifiedByNone"`
	Answer         string `json:"answer" enums:",None,Left,Right"`
	// Deprecated
	WrongWords      bool          `json:"wrongWords"`
	WrongWordsVotes uint32        `json:"wrongWordsVotes"`
	Timestamp       time.Time     `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	Size            uint32        `json:"size"`
	Icon            hexutil.Bytes `json:"icon,omitempty"`
	Words           *FlipWords    `json:"words"`
	WithPrivatePart bool          `json:"withPrivatePart"`
	Grade           byte          `json:"grade"`
	GradeScore      float32       `json:"gradeScore"`
} // @Name FlipSummary

type FlipWords struct {
	Word1 FlipWord `json:"word1"`
	Word2 FlipWord `json:"word2"`
} // @Name FlipWords

func (w FlipWords) IsEmpty() bool {
	return w.Word1.isEmpty() && w.Word2.isEmpty()
}

type FlipWord struct {
	Index uint16 `json:"index"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`
} // @Name FlipWord

func (w FlipWord) isEmpty() bool {
	return w.Index == 0 && len(w.Name) == 0 && len(w.Desc) == 0
}

// FlipAnswerCount mock type for swagger
type FlipAnswerCount struct {
	Value string `json:"value" enums:",None,Left,Right"`
	Count uint32 `json:"count"`
} // @Name FlipAnswerCount

// FlipStateCount mock type for swagger
type FlipStateCount struct {
	Value string `json:"value" enums:",NotQualified,Qualified,WeaklyQualified,QualifiedByNone"`
	Count uint32 `json:"count"`
} // @Name FlipStateCount

// IdentityStateCount mock type for swagger
type IdentityStateCount struct {
	Value string `json:"value" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	Count uint32 `json:"count"`
} // @Name IdentityStateCount

// SavedInviteRewardCount mock type for swagger
type SavedInviteRewardCount struct {
	Value string `json:"value" enums:"SavedInvite,SavedInviteWin"`
	Count uint32 `json:"count"`
} // @Name SavedInviteRewardCount

type NullableBoolValueCount struct {
	Value *bool  `json:"value"`
	Count uint32 `json:"count"`
} // @Name NullableBoolValueCount

type EpochIdentity struct {
	Address               string                 `json:"address,omitempty"`
	Epoch                 uint64                 `json:"epoch,omitempty"`
	PrevState             string                 `json:"prevState" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	State                 string                 `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	ShortAnswers          IdentityAnswersSummary `json:"shortAnswers"`
	TotalShortAnswers     IdentityAnswersSummary `json:"totalShortAnswers"`
	LongAnswers           IdentityAnswersSummary `json:"longAnswers"`
	ShortAnswersCount     uint32                 `json:"shortAnswersCount"`
	LongAnswersCount      uint32                 `json:"longAnswersCount"`
	Approved              bool                   `json:"approved"`
	Missed                bool                   `json:"missed"`
	RequiredFlips         uint8                  `json:"requiredFlips"`
	MadeFlips             uint8                  `json:"madeFlips"`
	AvailableFlips        uint8                  `json:"availableFlips"`
	TotalValidationReward decimal.Decimal        `json:"totalValidationReward" swaggertype:"string"`
	BirthEpoch            uint64                 `json:"birthEpoch"`
} // @Name EpochIdentity

type TransactionSummary struct {
	Hash      string           `json:"hash"`
	Type      string           `json:"type,omitempty" enums:"SendTx,ActivationTx,InviteTx,KillTx,SubmitFlipTx,SubmitAnswersHashTx,SubmitShortAnswersTx,SubmitLongAnswersTx,EvidenceTx,OnlineStatusTx,KillInviteeTx,ChangeGodAddressTx,BurnTx,ChangeProfileTx,DeleteFlipTx,DeployContract,CallContract,TerminateContract,DelegateTx,UndelegateTx,KillDelegatorTx"`
	Timestamp *time.Time       `json:"timestamp,omitempty" example:"2020-01-01T00:00:00Z"`
	From      string           `json:"from,omitempty"`
	To        string           `json:"to,omitempty"`
	Amount    *decimal.Decimal `json:"amount,omitempty" swaggertype:"string"`
	Tips      *decimal.Decimal `json:"tips,omitempty" swaggertype:"string"`
	MaxFee    *decimal.Decimal `json:"maxFee,omitempty" swaggertype:"string"`
	Fee       *decimal.Decimal `json:"fee,omitempty" swaggertype:"string"`
	Size      uint32           `json:"size,omitempty"`
	Nonce     uint32           `json:"nonce,omitempty"`
	// Deprecated
	Transfer *decimal.Decimal `json:"transfer,omitempty" swaggerignore:"true"`
	Data     interface{}      `json:"data,omitempty"`

	TxReceipt *TxReceipt `json:"txReceipt,omitempty"`
} // @Name TransactionSummary

// TransactionSpecificData mock type for swagger
type TransactionSpecificData struct {
	Transfer     *decimal.Decimal `json:"transfer,omitempty" swaggertype:"string"`
	BecomeOnline bool             `json:"becomeOnline"`
} // @Name TransactionSpecificData

type TransactionDetail struct {
	Epoch       uint64          `json:"epoch"`
	BlockHeight uint64          `json:"blockHeight"`
	BlockHash   string          `json:"blockHash"`
	Hash        string          `json:"hash"`
	Type        string          `json:"type" enums:"SendTx,ActivationTx,InviteTx,KillTx,SubmitFlipTx,SubmitAnswersHashTx,SubmitShortAnswersTx,SubmitLongAnswersTx,EvidenceTx,OnlineStatusTx,KillInviteeTx,ChangeGodAddressTx,BurnTx,ChangeProfileTx,DeleteFlipTx,DeployContract,CallContract,TerminateContract,DelegateTx,UndelegateTx,KillDelegatorTx"`
	Timestamp   *time.Time      `json:"timestamp,omitempty" example:"2020-01-01T00:00:00Z"`
	From        string          `json:"from"`
	To          string          `json:"to,omitempty"`
	Amount      decimal.Decimal `json:"amount" swaggertype:"string"`
	Tips        decimal.Decimal `json:"tips" swaggertype:"string"`
	MaxFee      decimal.Decimal `json:"maxFee" swaggertype:"string"`
	Fee         decimal.Decimal `json:"fee" swaggertype:"string"`
	Size        uint32          `json:"size"`
	Nonce       uint32          `json:"nonce,omitempty"`
	// Deprecated
	Transfer *decimal.Decimal `json:"transfer,omitempty" swaggertype:"string"`
	Data     interface{}      `json:"data,omitempty"`

	TxReceipt *TxReceipt `json:"txReceipt,omitempty"`
} // @Name Transaction

type ActivationTxSpecificData struct {
	Transfer *string `json:"transfer,omitempty"`
}

type KillTxSpecificData = ActivationTxSpecificData

type KillInviteeTxSpecificData = ActivationTxSpecificData

type OnlineStatusTxSpecificData struct {
	// Deprecated
	BecomeOnlineOld bool `json:"BecomeOnline"`
	BecomeOnline    bool `json:"becomeOnline"`
}

type Invite struct {
	Hash                 string     `json:"hash"`
	Author               string     `json:"author"`
	Timestamp            time.Time  `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	Epoch                uint64     `json:"epoch"`
	ActivationHash       string     `json:"activationHash"`
	ActivationAuthor     string     `json:"activationAuthor"`
	ActivationTimestamp  *time.Time `json:"activationTimestamp" example:"2020-01-01T00:00:00Z"`
	State                string     `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	KillInviteeHash      string     `json:"killInviteeHash,omitempty"`
	KillInviteeTimestamp *time.Time `json:"killInviteeTimestamp,omitempty" example:"2020-01-01T00:00:00Z"`
	KillInviteeEpoch     uint64     `json:"killInviteeEpoch,omitempty"`
} // @Name Invite

type Flip struct {
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	Size      uint32    `json:"size"`
	Status    string    `json:"status" enums:",NotQualified,Qualified,WeaklyQualified,QualifiedByNone"`
	Answer    string    `json:"answer" enums:",None,Left,Right"`
	// Deprecated
	WrongWords      bool       `json:"wrongWords"`
	WrongWordsVotes uint32     `json:"wrongWordsVotes"`
	TxHash          string     `json:"txHash"`
	BlockHash       string     `json:"blockHash"`
	BlockHeight     uint64     `json:"blockHeight"`
	Epoch           uint64     `json:"epoch"`
	Words           *FlipWords `json:"words"`
	WithPrivatePart bool       `json:"withPrivatePart"`
	Grade           byte       `json:"grade"`
	GradeScore      float32    `json:"gradeScore"`
} // @Name Flip

type FlipContent struct {
	LeftOrder  []uint16        `json:"leftOrder"`
	RightOrder []uint16        `json:"rightOrder"`
	Pics       []hexutil.Bytes `json:"pics" swaggertype:"array"`
	// Deprecated
	LeftOrderOld []uint16 `json:"LeftOrder" swaggerignore:"true"`
	// Deprecated
	RightOrderOld []uint16 `json:"RightOrder" swaggerignore:"true"`
	// Deprecated
	PicsOld []hexutil.Bytes `json:"Pics" swaggerignore:"true"`
} // @Name FlipContent

type Answer struct {
	Cid        string `json:"cid,omitempty"`
	Address    string `json:"address,omitempty"`
	RespAnswer string `json:"respAnswer" enums:"None,Left,Right"`
	// Deprecated
	RespWrongWords bool   `json:"respWrongWords"`
	FlipAnswer     string `json:"flipAnswer" enums:"None,Left,Right"`
	// Deprecated
	FlipWrongWords bool    `json:"flipWrongWords"`
	FlipStatus     string  `json:"flipStatus" enums:"NotQualified,Qualified,WeaklyQualified,QualifiedByNone"`
	Point          float32 `json:"point"`
	RespGrade      byte    `json:"respGrade"`
	FlipGrade      byte    `json:"flipGrade"`
	Index          byte    `json:"index"`
	Considered     bool    `json:"considered"`
	GradeIgnored   bool    `json:"gradeIgnored"`
} // @Name Answer

type Identity struct {
	Address           string                 `json:"address"`
	State             string                 `json:"state"`
	TotalShortAnswers IdentityAnswersSummary `json:"totalShortAnswers"`
} // @Name Identity

type IdentityFlipsSummary struct {
	States  []StrValueCount `json:"states"`
	Answers []StrValueCount `json:"answers"`
}

type IdentityAnswersSummary struct {
	Point      float32 `json:"point"`
	FlipsCount uint32  `json:"flipsCount"`
} // @Name IdentityAnswersSummary

type InvitesSummary struct {
	AllCount  uint64 `json:"allCount"`
	UsedCount uint64 `json:"usedCount"`
} // @Name InvitesSummary

type Address struct {
	Address            string          `json:"address"`
	Balance            decimal.Decimal `json:"balance" swaggertype:"string"`
	Stake              decimal.Decimal `json:"stake" swaggertype:"string"`
	TxCount            uint32          `json:"txCount"`
	FlipsCount         uint32          `json:"flipsCount"`
	ReportedFlipsCount uint32          `json:"reportedFlipsCount"`
	TokenCount         uint32          `json:"tokenCount"`
} // @Name Address

type Balance struct {
	Address string          `json:"address"`
	Balance decimal.Decimal `json:"balance" swaggertype:"string"`
	Stake   decimal.Decimal `json:"stake" swaggertype:"string"`
} // @Name Balance

type Staking struct {
	Weight             float64 `json:"weight"`
	MinersWeight       float64 `json:"minersWeight"`
	AverageMinerWeight float64 `json:"averageMinerWeight"`
	MaxMinerWeight     float64 `json:"maxMinerWeight"`
	ExtraFlipsWeight   float64 `json:"extraFlipsWeight"`
	InvitationsWeight  float64 `json:"invitationsWeight"`
} // @Name Staking

type TotalMiningReward struct {
	Address        string          `json:"address,omitempty"`
	Balance        decimal.Decimal `json:"balance"`
	Stake          decimal.Decimal `json:"stake"`
	Proposer       uint64          `json:"proposer"`
	FinalCommittee uint64          `json:"finalCommittee"`
}

type Penalty struct {
	Address        string          `json:"address"`
	Penalty        decimal.Decimal `json:"penalty" swaggertype:"string"`
	PenaltySeconds uint16          `json:"penaltySeconds"`
	BlockHeight    uint64          `json:"blockHeight"`
	BlockHash      string          `json:"blockHash"`
	Timestamp      time.Time       `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	Epoch          uint64          `json:"epoch"`
} // @Name Penalty

type Reward struct {
	Address     string          `json:"address,omitempty"`
	Epoch       uint64          `json:"epoch,omitempty"`
	BlockHeight uint64          `json:"blockHeight,omitempty"`
	Balance     decimal.Decimal `json:"balance" swaggertype:"string"`
	Stake       decimal.Decimal `json:"stake" swaggertype:"string"`
	Type        string          `json:"type" enums:"Validation,Flips,Invitations,Invitations2,Invitations3,SavedInvite,SavedInviteWin,Candidate,Staking,Invitee,Invitee2,Invitee3,ExtraFlips"`
} // @Name Reward

type Rewards struct {
	Address   string   `json:"address,omitempty"`
	Epoch     uint64   `json:"epoch,omitempty"`
	PrevState string   `json:"prevState" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	State     string   `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	Age       uint16   `json:"age"`
	Rewards   []Reward `json:"rewards"`
} // @Name Rewards

type FundPayment struct {
	Address string          `json:"address"`
	Balance decimal.Decimal `json:"balance" swaggertype:"string"`
	Type    string          `json:"type" enums:"FoundationPayouts,ZeroWalletFund"`
} // @Name FundPayment

type RewardBounds struct {
	Type byte        `json:"type" enums:"1,2,3,4,5,6"`
	Min  RewardBound `json:"min"`
	Max  RewardBound `json:"max"`
} // @Name RewardBounds

type RewardBound struct {
	Amount  decimal.Decimal `json:"amount" swaggertype:"string"`
	Address string          `json:"address"`
} // @Name RewardBound

type AddressState struct {
	State        string    `json:"state"`
	Epoch        uint64    `json:"epoch"`
	BlockHeight  uint64    `json:"blockHeight"`
	BlockHash    string    `json:"blockHash"`
	TxHash       string    `json:"txHash,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	IsValidation bool      `json:"isValidation"`
}

type AddressBurntCoins struct {
	Address string          `json:"address,omitempty"`
	Amount  decimal.Decimal `json:"amount"`
}

type BadAuthor struct {
	Epoch      uint64 `json:"epoch,omitempty"`
	Address    string `json:"address,omitempty"`
	WrongWords bool   `json:"wrongWords"`
	Reason     string `json:"reason" enums:"NoQualifiedFlips,QualifiedByNone,WrongWords"`
	PrevState  string `json:"prevState" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	State      string `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
} // @Name BadAuthor

type AdjacentStrValues struct {
	Prev AdjacentStrValue `json:"prev"`
	Next AdjacentStrValue `json:"next"`
}

type AdjacentStrValue struct {
	Value  string `json:"value"`
	Cycled bool   `json:"cycled"`
}

type FlipWithRewardFlag struct {
	FlipSummary
	Rewarded bool `json:"rewarded"`
} // @Name RewardedFlip

type ReportedFlipReward struct {
	Cid     string          `json:"cid"`
	Icon    hexutil.Bytes   `json:"icon,omitempty"`
	Author  string          `json:"author"`
	Words   *FlipWords      `json:"words"`
	Balance decimal.Decimal `json:"balance" swaggertype:"string"`
	Stake   decimal.Decimal `json:"stake" swaggertype:"string"`
	Grade   byte            `json:"grade"`
} // @Name ReportedFlipReward

type InviteWithRewardFlag struct {
	Invite
	RewardType  string `json:"rewardType,omitempty" enums:",Invitations,Invitations2,Invitations3"`
	EpochHeight uint32 `json:"epochHeight,omitempty"`
} // @Name RewardedInvite

type InviteeWithRewardFlag struct {
	Epoch        uint64          `json:"epoch"`
	Hash         string          `json:"hash"`
	Inviter      string          `json:"inviter"`
	InviterStake decimal.Decimal `json:"inviterStake" swaggertype:"string"`
	InviterState string          `json:"inviterState" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	State        string          `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
}

type EpochInvites struct {
	Epoch   uint64 `json:"epoch"`
	Invites uint8  `json:"invites"`
} // @Name EpochInvites

type BalanceUpdate struct {
	BalanceOld        decimal.Decimal `json:"balanceOld"`
	StakeOld          decimal.Decimal `json:"stakeOld"`
	PenaltyOld        decimal.Decimal `json:"penaltyOld"`
	PenaltySecondsOld uint16          `json:"penaltySecondsOld"`
	BalanceNew        decimal.Decimal `json:"balanceNew"`
	StakeNew          decimal.Decimal `json:"stakeNew"`
	PenaltyNew        decimal.Decimal `json:"penaltyNew"`
	PenaltySecondsNew uint16          `json:"penaltySecondsNew"`
	PenaltyPayment    decimal.Decimal `json:"penaltyPayment"`
	Reason            string          `json:"reason"`
	BlockHeight       uint64          `json:"blockHeight"`
	BlockHash         string          `json:"blockHash"`
	Timestamp         time.Time       `json:"timestamp"`
	Data              interface{}     `json:"data,omitempty"`
}

type TransactionBalanceUpdate struct {
	TxHash string `json:"txHash"`
}

type CommitteeRewardBalanceUpdate struct {
	LastBlockHeight    uint64          `json:"lastBlockHeight"`
	LastBlockHash      string          `json:"lastBlockHash"`
	LastBlockTimestamp time.Time       `json:"lastBlockTimestamp"`
	RewardShare        decimal.Decimal `json:"rewardShare"`
	BlocksCount        uint32          `json:"blocksCount"`
}

type ContractBalanceUpdate struct {
	TransactionBalanceUpdate
	ContractAddress string `json:"contractAddress"`
}

type EpochRewardBalanceUpdate struct {
	Epoch uint64 `json:"epoch"`
}

type BalanceUpdatesSummary struct {
	BalanceIn  decimal.Decimal `json:"balanceIn"`
	BalanceOut decimal.Decimal `json:"balanceOut"`
	StakeIn    decimal.Decimal `json:"stakeIn"`
	StakeOut   decimal.Decimal `json:"stakeOut"`
	PenaltyIn  decimal.Decimal `json:"penaltyIn"`
	PenaltyOut decimal.Decimal `json:"penaltyOut"`
} // @Name BalanceUpdatesSummary

type StrValueCount struct {
	Value string `json:"value"`
	Count uint32 `json:"count"`
}

type Contract struct {
	Address       string                `json:"address"`
	Type          string                `json:"type" enums:"TimeLock,OracleVoting,OracleLock,Multisig,RefundableOracleLock,Contract"`
	Author        string                `json:"author"`
	DeployTx      TransactionSummary    `json:"deployTx"`
	TerminationTx *TransactionSummary   `json:"terminationTx,omitempty"`
	Code          hexutil.Bytes         `json:"code,omitempty"`
	Verification  *ContractVerification `json:"verification,omitempty"`
	Token         *Token                `json:"token,omitempty"`
} // @Contract

type ContractVerification struct {
	State        string     `json:"state" enums:"Pending,Verified,Failed"`
	Timestamp    *time.Time `json:"timestamp,omitempty" example:"2020-01-01T00:00:00Z"`
	FileName     string     `json:"fileName,omitempty"`
	FileSize     uint32     `json:"fileSize,omitempty"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
} // @ContractVerification

type TimeLockContract struct {
	Timestamp JSONTime `json:"timestamp" swaggertype:"string" example:"2020-01-01T00:00:00Z"`
} // @TimeLockContract

type MultisigContract struct {
	MinVotes uint8                    `json:"minVotes"`
	MaxVotes uint8                    `json:"maxVotes"`
	Signers  []MultisigContractSigner `json:"signers,omitempty"`
} // @MultisigContract

type MultisigContractSigner struct {
	Address     string          `json:"address"`
	DestAddress string          `json:"destAddress"`
	Amount      decimal.Decimal `json:"amount" swaggertype:"string"`
} // @MultisigContractSigner

type OracleLockContract struct {
	OracleVotingAddress string `json:"oracleVotingAddress"`
	Value               byte   `json:"value"`
	SuccessAddress      string `json:"successAddress"`
	FailAddress         string `json:"failAddress"`
} // @OracleLockContract

type RefundableOracleLockContract struct {
	OracleVotingAddress string    `json:"oracleVotingAddress"`
	Value               byte      `json:"value"`
	SuccessAddress      string    `json:"successAddress"`
	FailAddress         string    `json:"failAddress"`
	DepositDeadline     time.Time `json:"depositDeadline" example:"2020-01-01T00:00:00Z"`
	OracleVotingFee     float32   `json:"oracleVotingFee"`
	RefundDelay         uint64    `json:"refundDelay"`
	RefundBlock         uint64    `json:"refundBlock,omitempty"`
	RefundDelayLeft     uint64    `json:"refundDelayLeft,omitempty"`
} // @RefundableOracleLockContract

type OracleVotingContract struct {
	ContractAddress                 string                            `json:"contractAddress"`
	Author                          string                            `json:"author"`
	Balance                         decimal.Decimal                   `json:"balance" swaggertype:"string"`
	Stake                           decimal.Decimal                   `json:"stake" swaggertype:"string"`
	Fact                            hexutil.Bytes                     `json:"fact"`
	VoteProofsCount                 uint64                            `json:"voteProofsCount"`
	SecretVotesCount                uint64                            `json:"secretVotesCount"`
	VotesCount                      uint64                            `json:"votesCount"`
	Votes                           []OracleVotingContractOptionVotes `json:"votes,omitempty"`
	State                           string                            `json:"state" enums:"Pending,Open,Voted,Counting,Archive,Terminated,CanBeProlonged"`
	CreateTime                      time.Time                         `json:"createTime" example:"2020-01-01T00:00:00Z"`
	StartTime                       time.Time                         `json:"startTime" example:"2020-01-01T00:00:00Z"`
	EstimatedVotingFinishTime       *time.Time                        `json:"estimatedVotingFinishTime,omitempty" example:"2020-01-01T00:00:00Z"`
	EstimatedPublicVotingFinishTime *time.Time                        `json:"estimatedPublicVotingFinishTime,omitempty" example:"2020-01-01T00:00:00Z"`
	EstimatedTerminationTime        *time.Time                        `json:"estimatedTerminationTime,omitempty" example:"2020-01-01T00:00:00Z"`
	EstimatedOracleReward           *decimal.Decimal                  `json:"estimatedOracleReward,omitempty" swaggertype:"string"`
	EstimatedMaxOracleReward        *decimal.Decimal                  `json:"estimatedMaxOracleReward,omitempty" swaggertype:"string"`
	EstimatedTotalReward            *decimal.Decimal                  `json:"estimatedTotalReward,omitempty" swaggertype:"string"`
	VotingFinishTime                *time.Time                        `json:"votingFinishTime,omitempty" example:"2020-01-01T00:00:00Z"`
	PublicVotingFinishTime          *time.Time                        `json:"publicVotingFinishTime,omitempty" example:"2020-01-01T00:00:00Z"`
	FinishTime                      *time.Time                        `json:"finishTime,omitempty" example:"2020-01-01T00:00:00Z"`
	TerminationTime                 *time.Time                        `json:"terminationTime,omitempty" example:"2020-01-01T00:00:00Z"`
	MinPayment                      *decimal.Decimal                  `json:"minPayment,omitempty" swaggertype:"string"`
	Quorum                          byte                              `json:"quorum"`
	CommitteeSize                   uint64                            `json:"committeeSize"`
	VotingDuration                  uint64                            `json:"votingDuration"`
	PublicVotingDuration            uint64                            `json:"publicVotingDuration"`
	WinnerThreshold                 byte                              `json:"winnerThreshold"`
	OwnerFee                        uint8                             `json:"ownerFee"`
	IsOracle                        bool                              `json:"isOracle"`
	CommitteeEpoch                  *uint64                           `json:"committeeEpoch,omitempty"`
	TotalReward                     *decimal.Decimal                  `json:"totalReward,omitempty" swaggertype:"string"`
	EpochWithoutGrowth              byte                              `json:"epochWithoutGrowth"`
	OwnerDeposit                    *decimal.Decimal                  `json:"ownerDeposit,omitempty" swaggertype:"string"`
	OracleRewardFund                *decimal.Decimal                  `json:"oracleRewardFund,omitempty" swaggertype:"string"`
	RefundRecipient                 string                            `json:"refundRecipient,omitempty" swaggertype:"string"`
	Hash                            hexutil.Bytes                     `json:"hash"`
} // @Name OracleVotingContract

type OracleVotingContractOptionVotes struct {
	Option   byte   `json:"option"`
	Count    uint64 `json:"count"`
	AllCount uint64 `json:"allCount"`
} // @Name OracleVotingContractOptionVotes

type EstimatedOracleReward struct {
	Amount decimal.Decimal `json:"amount" swaggertype:"string"`
	Type   string          `json:"type" enums:"min,low,medium,high,highest"`
} // @Name EstimatedOracleReward

type ContractTxBalanceUpdate struct {
	Hash      string          `json:"hash"`
	Type      string          `json:"type" enums:"SendTx,DeployContract,CallContract,TerminateContract"`
	Timestamp time.Time       `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	From      string          `json:"from"`
	To        string          `json:"to,omitempty"`
	Amount    decimal.Decimal `json:"amount" swaggertype:"string"`
	Tips      decimal.Decimal `json:"tips" swaggertype:"string"`
	MaxFee    decimal.Decimal `json:"maxFee" swaggertype:"string"`
	Fee       decimal.Decimal `json:"fee" swaggertype:"string"`

	Address         string           `json:"address"`
	ContractAddress string           `json:"contractAddress"`
	ContractType    string           `json:"contractType"`
	BalanceChange   *decimal.Decimal `json:"balanceChange,omitempty" swaggertype:"string"`

	TxReceipt *TxReceipt `json:"txReceipt,omitempty"`
} // @Name ContractTxBalanceUpdate

type TxReceipt struct {
	Success         bool            `json:"success"`
	GasUsed         uint64          `json:"gasUsed"`
	GasCost         decimal.Decimal `json:"gasCost" swaggertype:"string"`
	Method          string          `json:"method,omitempty"`
	ErrorMsg        string          `json:"errorMsg,omitempty"`
	ContractAddress string          `json:"contractAddress,omitempty"`
	ActionResult    *ActionResult   `json:"actionResult,omitempty"`
} // @Name TxReceipt

type ActionResult struct {
	InputAction      InputAction     `json:"inputAction"`
	Success          bool            `json:"success"`
	Error            string          `json:"error"`
	GasUsed          uint64          `json:"gasUsed"`
	RemainingGas     uint64          `json:"remainingGas"`
	OutputData       hexutil.Bytes   `json:"outputData"`
	SubActionResults []*ActionResult `json:"subActionResults"`
} // @Name ActionResult

type InputAction struct {
	ActionType uint32        `json:"actionType"`
	Amount     hexutil.Bytes `json:"amount"`
	Method     string        `json:"method"`
	Args       hexutil.Bytes `json:"args"`
	GasLimit   uint64        `json:"gasLimit"`
} // @Name InputAction

type TxEvent struct {
	EventName string          `json:"eventName"`
	Data      []hexutil.Bytes `json:"data,omitempty" swaggertype:"array"`
} // @Name TxEvent

type UpgradeVotes struct {
	Upgrade uint32 `json:"upgrade"`
	Votes   uint64 `json:"votes"`
} // @Name UpgradeVotes

type UpgradeVotingHistoryItem struct {
	Timestamp time.Time `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	Votes     uint64    `json:"votes"`
} // @Name UpgradeVotingHistoryItem

type Upgrade struct {
	Upgrade             uint32     `json:"upgrade,omitempty"`
	StartActivationDate *time.Time `json:"startActivationDate,omitempty" example:"2020-01-01T00:00:00Z"`
	EndActivationDate   *time.Time `json:"endActivationDate,omitempty" example:"2020-01-01T00:00:00Z"`
	Url                 string     `json:"url,omitempty"`
} // @Name Upgrade

type ActivatedUpgrade struct {
	BlockSummary
	Url string `json:"url,omitempty"`
} // @Name ActivatedUpgrade

type OnlineIdentity struct {
	Address        string          `json:"address"`
	LastActivity   *time.Time      `json:"lastActivity"`
	Penalty        decimal.Decimal `json:"penalty"`
	PenaltySeconds uint16          `json:"penaltySeconds"`
	Online         bool            `json:"online"`
	Delegetee      *OnlineIdentity `json:"delegatee,omitempty"`
}

type Validator struct {
	Address        string          `json:"address"`
	Size           uint32          `json:"size"`
	Online         bool            `json:"online"`
	LastActivity   *time.Time      `json:"lastActivity"`
	Penalty        decimal.Decimal `json:"penalty"`
	PenaltySeconds uint16          `json:"penaltySeconds"`
	IsPool         bool            `json:"isPool"`
}

type Pool struct {
	Address             string          `json:"address"`
	Size                uint64          `json:"size"`
	TotalStake          decimal.Decimal `json:"totalStake" swaggertype:"string"`
	TotalValidatedStake decimal.Decimal `json:"totalValidatedStake" swaggertype:"string"`
} // @Name Pool

type Delegator struct {
	Address string          `json:"address"`
	State   string          `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	Age     uint16          `json:"age"`
	Stake   decimal.Decimal `json:"stake" swaggertype:"string"`
} // @Name Delegator

type MinersHistoryItem struct {
	Timestamp        time.Time `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	OnlineMiners     uint64    `json:"onlineMiners"`
	OnlineValidators uint64    `json:"onlineValidators"`
} // @Name MinersHistoryItem

type PeersHistoryItem struct {
	Timestamp time.Time `json:"timestamp" example:"2020-01-01T00:00:00Z"`
	Count     uint64    `json:"count"`
} // @Name PeersHistoryItem

type DynamicEndpoint struct {
	Method     string
	DataSource string
	Limit      *int
}

type DynamicEndpointResult struct {
	Data []map[string]interface{} `json:"data"`
	Date *time.Time               `json:"date,omitempty"`
}

type DelegateeTotalRewards struct {
	Address             string             `json:"address,omitempty"`
	Epoch               uint64             `json:"epoch,omitempty"`
	Rewards             []DelegationReward `json:"rewards"`
	Delegators          uint32             `json:"delegators"`
	PenalizedDelegators uint32             `json:"penalizedDelegators"`
} // @Name DelegateeTotalRewards

type DelegateeReward struct {
	DelegatorAddress string             `json:"delegatorAddress"`
	PrevState        string             `json:"prevState" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	State            string             `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	Rewards          []DelegationReward `json:"rewards"`
} // @Name DelegateeReward

type DelegationReward struct {
	Balance decimal.Decimal `json:"balance" swaggertype:"string"`
	Type    string          `json:"type" enums:"Validation,Flips,Invitations,Invitations2,Invitations3,SavedInvite,SavedInviteWin,Reports,Candidate,Staking,Invitee,Invitee2,Invitee3,ExtraFlips"`
} // @Name DelegationReward

type ValidationSummary struct {
	MadeFlips         uint8                      `json:"madeFlips"`
	AvailableFlips    uint8                      `json:"availableFlips"`
	ShortAnswers      IdentityAnswersSummary     `json:"shortAnswers"`
	TotalShortAnswers IdentityAnswersSummary     `json:"totalShortAnswers"`
	LongAnswers       IdentityAnswersSummary     `json:"longAnswers"`
	ShortAnswersCount uint32                     `json:"shortAnswersCount"`
	LongAnswersCount  uint32                     `json:"longAnswersCount"`
	PrevState         string                     `json:"prevState" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	State             string                     `json:"state" enums:"Undefined,Invite,Candidate,Verified,Suspended,Killed,Zombie,Newbie,Human"`
	Penalized         bool                       `json:"penalized"`
	PenaltyReason     string                     `json:"penaltyReason,omitempty" enums:"NoQualifiedFlips,QualifiedByNone,WrongWords"`
	Approved          bool                       `json:"approved"`
	Missed            bool                       `json:"missed"`
	Rewards           ValidationRewardSummaries  `json:"rewards"`
	DelegateeReward   *ValidationDelegateeReward `json:"delegateeReward,omitempty"`
	Stake             decimal.Decimal            `json:"stake" swaggertype:"string"`
	WrongGrades       bool                       `json:"wrongGrades"`
} // @Name ValidationSummary

type ValidationRewardSummaries struct {
	Validation  ValidationRewardSummary `json:"validation"`
	Flips       ValidationRewardSummary `json:"flips"`
	ExtraFlips  ValidationRewardSummary `json:"extraFlips"`
	Invitations ValidationRewardSummary `json:"invitations"`
	Invitee     ValidationRewardSummary `json:"invitee"`
	Reports     ValidationRewardSummary `json:"reports"`
	Candidate   ValidationRewardSummary `json:"candidate"`
	Staking     ValidationRewardSummary `json:"staking"`
} // @Name ValidationRewardSummaries

type ValidationRewardSummary struct {
	Earned       decimal.Decimal `json:"earned" swaggertype:"string"`
	Missed       decimal.Decimal `json:"missed" swaggertype:"string"`
	MissedReason string          `json:"reason,omitempty"`
} // @Name ValidationRewardSummary

type ValidationDelegateeReward struct {
	Address string          `json:"address"`
	Amount  decimal.Decimal `json:"amount" swaggertype:"string"`
} // @Name ValidationDelegateeReward

type MiningRewardSummary struct {
	Epoch   uint64          `json:"epoch"`
	Amount  decimal.Decimal `json:"amount" swaggertype:"string"`
	Penalty decimal.Decimal `json:"penalty" swaggertype:"string"`
} // @Name MiningRewardSummary

type Token struct {
	ContractAddress string `json:"contractAddress"`
	Name            string `json:"name"`
	Symbol          string `json:"symbol"`
	Decimals        byte   `json:"decimals"`
} // @Name Token

type TokenBalance struct {
	Token   Token           `json:"token"`
	Address string          `json:"address"`
	Balance decimal.Decimal `json:"balance" swaggertype:"string"`
} // @Name TokenBalance

type Delegation struct {
	DelegateeAddress   string              `json:"delegateeAddress"`
	DelegationTx       TransactionSummary  `json:"delegationTx"`
	DelegationBlock    *BlockSummary       `json:"delegationBlock,omitempty"`
	UndelegationTx     *TransactionSummary `json:"undelegationTx,omitempty"`
	UndelegationBlock  *BlockSummary       `json:"undelegationBlock,omitempty"`
	UndelegationReason string              `json:"undelegationReason,omitempty" enums:"Undelegation,Termination,ValidationFailure,TransitionRemove,InactiveIdentity"`
} // @Name Delegation

type PoolSizeHistoryItem struct {
	Epoch          uint64 `json:"epoch"`
	StartSize      uint64 `json:"startSize"`
	ValidationSize uint64 `json:"validationSize"`
	EndSize        uint64 `json:"endSize"`
} // @Name PoolSizeHistoryItem
