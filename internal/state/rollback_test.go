package state_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

func TestRollback(t *testing.T) {
	var (
		height     int64 = 100
		nextHeight int64 = 101
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	blockStore := &mocks.BlockStore{}
	stateStore := setupStateStore(ctx, t, height)
	initialState, err := stateStore.Load()
	require.NoError(t, err)

	// perform the rollback over a version bump
	newParams := types.DefaultConsensusParams()
	newParams.Version.AppVersion = 11
	newParams.Block.MaxBytes = 1000
	nextState := initialState.Copy()
	nextState.LastBlockHeight = nextHeight
	nextState.Version.Consensus.App = 11
	nextState.LastBlockID = factory.MakeBlockID()
	nextState.AppHash = factory.RandomHash()
	nextState.LastValidators = initialState.Validators
	nextState.Validators = initialState.NextValidators
	nextState.NextValidators = initialState.NextValidators.CopyIncrementProposerPriority(1)
	nextState.ConsensusParams = *newParams
	nextState.LastHeightConsensusParamsChanged = nextHeight + 1
	nextState.LastHeightValidatorsChanged = nextHeight + 1

	// update the state
	require.NoError(t, stateStore.Save(nextState))

	block := &types.BlockMeta{
		BlockID: initialState.LastBlockID,
		Header: types.Header{
			Height:          initialState.LastBlockHeight,
			AppHash:         initialState.AppHash,
			LastBlockID:     factory.MakeBlockID(),
			LastResultsHash: initialState.LastResultsHash,
		},
	}
	blockStore.On("LoadBlockMeta", initialState.LastBlockHeight).Return(block)
	blockStore.On("Height").Return(nextHeight)

	// rollback the state
	rollbackHeight, rollbackHash, err := state.Rollback(blockStore, stateStore)
	require.NoError(t, err)
	require.EqualValues(t, height, rollbackHeight)
	require.EqualValues(t, initialState.AppHash, rollbackHash)
	blockStore.AssertExpectations(t)

	// assert that we've recovered the prior state
	loadedState, err := stateStore.Load()
	require.NoError(t, err)
	require.EqualValues(t, initialState, loadedState)
}

func TestRollbackNoState(t *testing.T) {
	stateStore := state.NewStore(dbm.NewMemDB())
	blockStore := &mocks.BlockStore{}

	_, _, err := state.Rollback(blockStore, stateStore)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no state found")
}

func TestRollbackNoBlocks(t *testing.T) {
	const height = int64(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateStore := setupStateStore(ctx, t, height)
	blockStore := &mocks.BlockStore{}
	blockStore.On("Height").Return(height)
	blockStore.On("LoadBlockMeta", height-1).Return(nil)

	_, _, err := state.Rollback(blockStore, stateStore)
	require.Error(t, err)
	require.Contains(t, err.Error(), "block at height 99 not found")
}

func TestRollbackDifferentStateHeight(t *testing.T) {
	const height = int64(100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateStore := setupStateStore(ctx, t, height)
	blockStore := &mocks.BlockStore{}
	blockStore.On("Height").Return(height + 2)

	_, _, err := state.Rollback(blockStore, stateStore)
	require.Error(t, err)
	require.Equal(t, err.Error(), "statestore height (100) is not one below or equal to blockstore height (102)")
}

func setupStateStore(ctx context.Context, t *testing.T, height int64) state.Store {
	stateStore := state.NewStore(dbm.NewMemDB())
	valSet, _ := factory.RandValidatorSet(ctx, 5, 10)

	params := types.DefaultConsensusParams()
	params.Version.AppVersion = 10

	initialState := state.State{
		Version: state.Version{
			Consensus: version.Consensus{
				Block: version.BlockProtocol,
				App:   10,
			},
			Software: version.TMVersion,
		},
		ChainID:                          factory.DefaultTestChainID,
		InitialHeight:                    10,
		LastBlockID:                      factory.MakeBlockID(),
		AppHash:                          factory.RandomHash(),
		LastResultsHash:                  factory.RandomHash(),
		LastBlockHeight:                  height,
		LastValidators:                   valSet,
		Validators:                       valSet.CopyIncrementProposerPriority(1),
		NextValidators:                   valSet.CopyIncrementProposerPriority(2),
		LastHeightValidatorsChanged:      height + 1,
		ConsensusParams:                  *params,
		LastHeightConsensusParamsChanged: height + 1,
	}
	require.NoError(t, stateStore.Bootstrap(initialState))
	return stateStore
}
