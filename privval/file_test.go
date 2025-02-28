package privval

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestGenLoadValidator(t *testing.T) {
	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.NoError(t, err)

	privVal, err := GenFilePV(tempKeyFile.Name(), tempStateFile.Name(), "")
	require.NoError(t, err)

	height := int64(100)
	privVal.LastSignState.Height = height
	require.NoError(t, privVal.Save())
	addr := privVal.GetAddress()

	privVal, err = LoadFilePV(tempKeyFile.Name(), tempStateFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, addr, privVal.GetAddress(), "expected privval addr to be the same")
	assert.Equal(t, height, privVal.LastSignState.Height, "expected privval.LastHeight to have been saved")
}

func TestResetValidator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.NoError(t, err)

	privVal, err := GenFilePV(tempKeyFile.Name(), tempStateFile.Name(), "")
	require.NoError(t, err)
	emptyState := FilePVLastSignState{filePath: tempStateFile.Name()}

	// new priv val has empty state
	assert.Equal(t, privVal.LastSignState, emptyState)

	// test vote
	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType
	randBytes := tmrand.Bytes(tmhash.Size)
	blockID := types.BlockID{Hash: randBytes, PartSetHeader: types.PartSetHeader{}}
	vote := newVote(privVal.Key.Address, 0, height, round, voteType, blockID)
	err = privVal.SignVote(ctx, "mychainid", vote.ToProto())
	assert.NoError(t, err, "expected no error signing vote")

	// priv val after signing is not same as empty
	assert.NotEqual(t, privVal.LastSignState, emptyState)

	// priv val after AcceptNewConnection is same as empty
	require.NoError(t, privVal.Reset())
	assert.Equal(t, privVal.LastSignState, emptyState)
}

func TestLoadOrGenValidator(t *testing.T) {
	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.NoError(t, err)

	tempKeyFilePath := tempKeyFile.Name()
	if err := os.Remove(tempKeyFilePath); err != nil {
		t.Error(err)
	}
	tempStateFilePath := tempStateFile.Name()
	if err := os.Remove(tempStateFilePath); err != nil {
		t.Error(err)
	}

	privVal, err := LoadOrGenFilePV(tempKeyFilePath, tempStateFilePath)
	require.NoError(t, err)
	addr := privVal.GetAddress()
	privVal, err = LoadOrGenFilePV(tempKeyFilePath, tempStateFilePath)
	require.NoError(t, err)
	assert.Equal(t, addr, privVal.GetAddress(), "expected privval addr to be the same")
}

func TestUnmarshalValidatorState(t *testing.T) {
	// create some fixed values
	serialized := `{
		"height": "1",
		"round": 1,
		"step": 1
	}`

	val := FilePVLastSignState{}
	err := tmjson.Unmarshal([]byte(serialized), &val)
	require.NoError(t, err)

	// make sure the values match
	assert.EqualValues(t, val.Height, 1)
	assert.EqualValues(t, val.Round, 1)
	assert.EqualValues(t, val.Step, 1)

	// export it and make sure it is the same
	out, err := tmjson.Marshal(val)
	require.NoError(t, err)
	assert.JSONEq(t, serialized, string(out))
}

func TestUnmarshalValidatorKey(t *testing.T) {
	// create some fixed values
	privKey := ed25519.GenPrivKey()
	pubKey := privKey.PubKey()
	addr := pubKey.Address()
	pubBytes := pubKey.Bytes()
	privBytes := privKey.Bytes()
	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)
	privB64 := base64.StdEncoding.EncodeToString(privBytes)

	serialized := fmt.Sprintf(`{
  "address": "%s",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "%s"
  },
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "%s"
  }
}`, addr, pubB64, privB64)

	val := FilePVKey{}
	err := tmjson.Unmarshal([]byte(serialized), &val)
	require.NoError(t, err)

	// make sure the values match
	assert.EqualValues(t, addr, val.Address)
	assert.EqualValues(t, pubKey, val.PubKey)
	assert.EqualValues(t, privKey, val.PrivKey)

	// export it and make sure it is the same
	out, err := tmjson.Marshal(val)
	require.NoError(t, err)
	assert.JSONEq(t, serialized, string(out))
}

func TestSignVote(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.NoError(t, err)

	privVal, err := GenFilePV(tempKeyFile.Name(), tempStateFile.Name(), "")
	require.NoError(t, err)

	randbytes := tmrand.Bytes(tmhash.Size)
	randbytes2 := tmrand.Bytes(tmhash.Size)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}

	height, round := int64(10), int32(1)
	voteType := tmproto.PrevoteType

	// sign a vote for first time
	vote := newVote(privVal.Key.Address, 0, height, round, voteType, block1)
	v := vote.ToProto()

	err = privVal.SignVote(ctx, "mychainid", v)
	assert.NoError(t, err, "expected no error signing vote")

	// try to sign the same vote again; should be fine
	err = privVal.SignVote(ctx, "mychainid", v)
	assert.NoError(t, err, "expected no error on signing same vote")

	// now try some bad votes
	cases := []*types.Vote{
		newVote(privVal.Key.Address, 0, height, round-1, voteType, block1),   // round regression
		newVote(privVal.Key.Address, 0, height-1, round, voteType, block1),   // height regression
		newVote(privVal.Key.Address, 0, height-2, round+4, voteType, block1), // height regression and different round
		newVote(privVal.Key.Address, 0, height, round, voteType, block2),     // different block
	}

	for _, c := range cases {
		assert.Error(t, privVal.SignVote(ctx, "mychainid", c.ToProto()),
			"expected error on signing conflicting vote")
	}

	// try signing a vote with a different time stamp
	sig := vote.Signature
	vote.Timestamp = vote.Timestamp.Add(time.Duration(1000))
	err = privVal.SignVote(ctx, "mychainid", v)
	assert.NoError(t, err)
	assert.Equal(t, sig, vote.Signature)
}

func TestSignProposal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.NoError(t, err)

	privVal, err := GenFilePV(tempKeyFile.Name(), tempStateFile.Name(), "")
	require.NoError(t, err)

	randbytes := tmrand.Bytes(tmhash.Size)
	randbytes2 := tmrand.Bytes(tmhash.Size)

	block1 := types.BlockID{Hash: randbytes,
		PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	block2 := types.BlockID{Hash: randbytes2,
		PartSetHeader: types.PartSetHeader{Total: 10, Hash: randbytes2}}
	height, round := int64(10), int32(1)

	// sign a proposal for first time
	proposal := newProposal(height, round, block1)
	pbp := proposal.ToProto()

	err = privVal.SignProposal(ctx, "mychainid", pbp)
	assert.NoError(t, err, "expected no error signing proposal")

	// try to sign the same proposal again; should be fine
	err = privVal.SignProposal(ctx, "mychainid", pbp)
	assert.NoError(t, err, "expected no error on signing same proposal")

	// now try some bad Proposals
	cases := []*types.Proposal{
		newProposal(height, round-1, block1),   // round regression
		newProposal(height-1, round, block1),   // height regression
		newProposal(height-2, round+4, block1), // height regression and different round
		newProposal(height, round, block2),     // different block
	}

	for _, c := range cases {
		assert.Error(t, privVal.SignProposal(ctx, "mychainid", c.ToProto()),
			"expected error on signing conflicting proposal")
	}

	// try signing a proposal with a different time stamp
	sig := proposal.Signature
	proposal.Timestamp = proposal.Timestamp.Add(time.Duration(1000))
	err = privVal.SignProposal(ctx, "mychainid", pbp)
	assert.NoError(t, err)
	assert.Equal(t, sig, proposal.Signature)
}

func TestDifferByTimestamp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
	require.NoError(t, err)
	tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
	require.NoError(t, err)

	privVal, err := GenFilePV(tempKeyFile.Name(), tempStateFile.Name(), "")
	require.NoError(t, err)
	randbytes := tmrand.Bytes(tmhash.Size)
	block1 := types.BlockID{Hash: randbytes, PartSetHeader: types.PartSetHeader{Total: 5, Hash: randbytes}}
	height, round := int64(10), int32(1)
	chainID := "mychainid"

	// test proposal
	{
		proposal := newProposal(height, round, block1)
		pb := proposal.ToProto()
		err := privVal.SignProposal(ctx, chainID, pb)
		require.NoError(t, err, "expected no error signing proposal")
		signBytes := types.ProposalSignBytes(chainID, pb)

		sig := proposal.Signature
		timeStamp := proposal.Timestamp

		// manipulate the timestamp. should get changed back
		pb.Timestamp = pb.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		proposal.Signature = emptySig
		err = privVal.SignProposal(ctx, "mychainid", pb)
		require.NoError(t, err, "expected no error on signing same proposal")

		assert.Equal(t, timeStamp, pb.Timestamp)
		assert.Equal(t, signBytes, types.ProposalSignBytes(chainID, pb))
		assert.Equal(t, sig, proposal.Signature)
	}

	// test vote
	{
		voteType := tmproto.PrevoteType
		blockID := types.BlockID{Hash: randbytes, PartSetHeader: types.PartSetHeader{}}
		vote := newVote(privVal.Key.Address, 0, height, round, voteType, blockID)
		v := vote.ToProto()
		err := privVal.SignVote(ctx, "mychainid", v)
		require.NoError(t, err, "expected no error signing vote")

		signBytes := types.VoteSignBytes(chainID, v)
		sig := v.Signature
		timeStamp := vote.Timestamp

		// manipulate the timestamp. should get changed back
		v.Timestamp = v.Timestamp.Add(time.Millisecond)
		var emptySig []byte
		v.Signature = emptySig
		err = privVal.SignVote(ctx, "mychainid", v)
		require.NoError(t, err, "expected no error on signing same vote")

		assert.Equal(t, timeStamp, v.Timestamp)
		assert.Equal(t, signBytes, types.VoteSignBytes(chainID, v))
		assert.Equal(t, sig, v.Signature)
	}
}

func newVote(addr types.Address, idx int32, height int64, round int32,
	typ tmproto.SignedMsgType, blockID types.BlockID) *types.Vote {
	return &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            round,
		Type:             typ,
		Timestamp:        tmtime.Now(),
		BlockID:          blockID,
	}
}

func newProposal(height int64, round int32, blockID types.BlockID) *types.Proposal {
	return &types.Proposal{
		Height:    height,
		Round:     round,
		BlockID:   blockID,
		Timestamp: tmtime.Now(),
	}
}
