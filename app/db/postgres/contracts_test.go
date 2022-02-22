package postgres

import (
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_calculateEstimatedMaxOracleReward(t *testing.T) {
	votingMinPayment := decimal.New(2, 0)
	res := calculateEstimatedMaxOracleReward(
		decimal.New(100, 0),
		&votingMinPayment,
		50,
		200,
		5,
		51,
		4)

	require.Equal(t, "11.0000", res.StringFixed(4))

	res = calculateEstimatedMaxOracleReward(
		decimal.New(10, 0),
		nil,
		0,
		10,
		1,
		66,
		0)

	require.Equal(t, "10.0000", res.StringFixed(4))

	res = calculateEstimatedMaxOracleReward(
		decimal.New(5000, 0),
		nil,
		95,
		4,
		5,
		51,
		1)
	require.Equal(t, "250.0000", res.StringFixed(4))
}
