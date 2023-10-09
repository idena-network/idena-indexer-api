package postgres

import (
	"github.com/idena-network/idena-indexer-api/app/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_extendPoolSizeHistory(t *testing.T) {
	p := func(v string) *string {
		return &v
	}

	{
		res, token := extendPoolSizeHistory([]types.PoolSizeHistoryItem{
			{Epoch: 110, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 107, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 103, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 102, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 101, StartSize: 1, ValidationSize: 1, EndSize: 1},
		}, 5, nil, p("100"))

		require.Len(t, res, 5)

		require.Equal(t, uint64(111), res[0].Epoch)
		require.Equal(t, uint64(1), res[0].StartSize)
		require.Equal(t, uint64(0), res[0].ValidationSize)
		require.Equal(t, uint64(0), res[0].EndSize)

		require.Equal(t, uint64(110), res[1].Epoch)
		require.Equal(t, uint64(1), res[1].StartSize)
		require.Equal(t, uint64(1), res[1].ValidationSize)
		require.Equal(t, uint64(1), res[1].EndSize)

		require.Equal(t, uint64(108), res[2].Epoch)
		require.Equal(t, uint64(107), res[3].Epoch)
		require.Equal(t, uint64(104), res[4].Epoch)
		require.NotNil(t, token)
		require.Equal(t, "103", *token)
	}

	{
		res, token := extendPoolSizeHistory([]types.PoolSizeHistoryItem{
			{Epoch: 110, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 107, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 103, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 102, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 101, StartSize: 1, ValidationSize: 1, EndSize: 1},
		}, 5, p("110"), p("100"))

		require.Len(t, res, 5)

		require.Equal(t, uint64(110), res[0].Epoch)
		require.Equal(t, uint64(1), res[0].StartSize)
		require.Equal(t, uint64(1), res[0].ValidationSize)
		require.Equal(t, uint64(1), res[0].EndSize)

		require.Equal(t, uint64(108), res[1].Epoch)
		require.Equal(t, uint64(107), res[2].Epoch)
		require.Equal(t, uint64(104), res[3].Epoch)
		require.Equal(t, uint64(103), res[4].Epoch)
		require.NotNil(t, token)
		require.Equal(t, "102", *token)
	}

	{
		res, token := extendPoolSizeHistory([]types.PoolSizeHistoryItem{
			{Epoch: 110, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 107, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 103, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 102, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 101, StartSize: 1, ValidationSize: 1, EndSize: 1},
		}, 10, nil, p("100"))

		require.Len(t, res, 8)

		require.Equal(t, uint64(111), res[0].Epoch)
		require.Equal(t, uint64(1), res[0].StartSize)
		require.Equal(t, uint64(0), res[0].ValidationSize)
		require.Equal(t, uint64(0), res[0].EndSize)

		require.Equal(t, uint64(110), res[1].Epoch)
		require.Equal(t, uint64(1), res[1].StartSize)
		require.Equal(t, uint64(1), res[1].ValidationSize)
		require.Equal(t, uint64(1), res[1].EndSize)

		require.Equal(t, uint64(108), res[2].Epoch)
		require.Equal(t, uint64(107), res[3].Epoch)
		require.Equal(t, uint64(104), res[4].Epoch)
		require.Equal(t, uint64(103), res[5].Epoch)
		require.Equal(t, uint64(102), res[6].Epoch)
		require.Equal(t, uint64(101), res[7].Epoch)
		require.NotNil(t, token)
		require.Equal(t, "100", *token)
	}

	{
		res, token := extendPoolSizeHistory([]types.PoolSizeHistoryItem{
			{Epoch: 110, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 107, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 103, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 102, StartSize: 1, ValidationSize: 1, EndSize: 1},
			{Epoch: 101, StartSize: 1, ValidationSize: 1, EndSize: 1},
		}, 10, p("110"), p("100"))

		require.Len(t, res, 7)

		require.Equal(t, uint64(110), res[0].Epoch)
		require.Equal(t, uint64(1), res[0].StartSize)
		require.Equal(t, uint64(1), res[0].ValidationSize)
		require.Equal(t, uint64(1), res[0].EndSize)

		require.Equal(t, uint64(108), res[1].Epoch)
		require.Equal(t, uint64(107), res[2].Epoch)
		require.Equal(t, uint64(104), res[3].Epoch)
		require.Equal(t, uint64(103), res[4].Epoch)
		require.Equal(t, uint64(102), res[5].Epoch)
		require.Equal(t, uint64(101), res[6].Epoch)
		require.NotNil(t, token)
		require.Equal(t, "100", *token)
	}

}
