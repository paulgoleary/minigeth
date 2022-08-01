package test

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/mpt"
	"github.com/ethereum/go-ethereum/oracle"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/stretchr/testify/require"

	"testing"
)

func TestAccountProof(t *testing.T) {
	oracle.SetNodeUrl("https://silent-polished-field.matic.discover.quiknode.pro/66d1e4f51172a3cb514ec76479899e83daade131/")

	blockNum := oracle.GetBlockNumber()
	header := oracle.GetBlockHeader(blockNum)

	addr := common.HexToAddress("268c2bbb09f62a6c278b2a43a35e9e546088d3a7")
	hash := common.HexToHash("000000000000000000000000000000000000000000000000000000000000000001")
	acctRet := oracle.GetProofAccount(blockNum, addr, hash, false)

	proofTrie := mpt.NewProofDB()

	for _, encNode := range acctRet {
		nodeBytes := hexutil.MustDecode(encNode)
		proofTrie.Put(crypto.Keccak256(nodeBytes), nodeBytes)
	}

	validAccountState, err := trie.VerifyProof(header.Root, crypto.Keccak256(addr.Bytes()), proofTrie)
	require.NoError(t, err)
	_ = validAccountState
}
