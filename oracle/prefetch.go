//go:build !mips
// +build !mips

package oracle

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/crypto/sha3"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type jsonreq struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      uint64        `json:"id"`
}

type jsonresp struct {
	Jsonrpc string        `json:"jsonrpc"`
	Id      uint64        `json:"id"`
	Result  AccountResult `json:"result"`
}

type jsonresps struct {
	Jsonrpc string                 `json:"jsonrpc"`
	Id      uint64                 `json:"id"`
	Result  string                 `json:"result"`
	Error   map[string]interface{} `json:"error"`
}

type jsonrespi struct {
	Jsonrpc string         `json:"jsonrpc"`
	Id      uint64         `json:"id"`
	Result  hexutil.Uint64 `json:"result"`
}

type jsonrespt struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      uint64 `json:"id"`
	Result  Header `json:"result"`
}

// Result structs for GetProof
type AccountResult struct {
	Address      common.Address  `json:"address"`
	AccountProof []string        `json:"accountProof"`
	Balance      *hexutil.Big    `json:"balance"`
	CodeHash     common.Hash     `json:"codeHash"`
	Nonce        hexutil.Uint64  `json:"nonce"`
	StorageHash  common.Hash     `json:"storageHash"`
	StorageProof []StorageResult `json:"storageProof"`
}

type StorageResult struct {
	Key   string       `json:"key"`
	Value *hexutil.Big `json:"value"`
	Proof []string     `json:"proof"`
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

// var nodeUrl = "https://mainnet.infura.io/v3/9aa3d95b3bc440fa88ea12eaa4456161"
// var nodeUrl = "http://ec2-52-53-239-185.us-west-1.compute.amazonaws.com:8545"
var nodeUrl = "https://polygon-rpc.com"

func SetNodeUrl(newNodeUrl string) {
	nodeUrl = newNodeUrl
}

func toFilename(key string) string {
	return fmt.Sprintf("%s/json_%s", root, key)
}

func cacheRead(key string) []byte {
	dat, err := ioutil.ReadFile(toFilename(key))
	if err == nil {
		return dat
	}
	panic("cache missing")
}

func cacheExists(key string) bool {
	_, err := os.Stat(toFilename(key))
	return err == nil
}

func cacheWrite(key string, value []byte) {
	ioutil.WriteFile(toFilename(key), value, 0644)
}

func getAPI(jsonData []byte) io.Reader {
	key := hexutil.Encode(crypto.Keccak256(jsonData))
	if cacheExists(key) {
		return bytes.NewReader(cacheRead(key))
	}
	if resp, err := http.Post(nodeUrl, "application/json", bytes.NewBuffer(jsonData)); err == nil {
		defer resp.Body.Close()
		ret, _ := ioutil.ReadAll(resp.Body)
		cacheWrite(key, ret)
		return bytes.NewReader(ret)
	} else {
		return nil
	}
}

var unhashMap = make(map[common.Hash]common.Address)

func unhash(addrHash common.Hash) common.Address {
	return unhashMap[addrHash]
}

var cached = make(map[string]bool)

func PrefetchStorage(blockNumber *big.Int, addr common.Address, skey common.Hash, postProcess func(map[common.Hash][]byte)) {
	key := fmt.Sprintf("proof_%d_%s_%s", blockNumber, addr, skey)
	if cached[key] {
		return
	}
	cached[key] = true

	ap := GetProofAccount(blockNumber, addr, skey, true)
	// fmt.Println("PrefetchStorage", blockNumber, addr, skey, len(ap))
	newPreimages := make(map[common.Hash][]byte)
	for _, s := range ap {
		ret, _ := hex.DecodeString(s[2:])
		hash := crypto.Keccak256Hash(ret)
		//fmt.Println("   ", i, hash)
		newPreimages[hash] = ret
	}

	if postProcess != nil {
		postProcess(newPreimages)
	}

	for hash, val := range newPreimages {
		preimages[hash] = val
	}
}

func PrefetchAccount(blockNumber *big.Int, addr common.Address, postProcess func(map[common.Hash][]byte)) {
	key := fmt.Sprintf("proof_%d_%s", blockNumber, addr)
	if cached[key] {
		return
	}
	cached[key] = true

	ap := GetProofAccount(blockNumber, addr, common.Hash{}, false)
	newPreimages := make(map[common.Hash][]byte)
	for _, s := range ap {
		ret, _ := hex.DecodeString(s[2:])
		hash := crypto.Keccak256Hash(ret)
		newPreimages[hash] = ret
	}

	if postProcess != nil {
		postProcess(newPreimages)
	}

	for hash, val := range newPreimages {
		preimages[hash] = val
	}
}

func PrefetchCode(blockNumber *big.Int, addrHash common.Hash) {
	key := fmt.Sprintf("code_%d_%s", blockNumber, addrHash)
	if cached[key] {
		return
	}
	cached[key] = true
	ret := getProvedCodeBytes(blockNumber, addrHash)
	hash := crypto.Keccak256Hash(ret)
	preimages[hash] = ret
}

var inputhash common.Hash

func InputHash() common.Hash {
	return inputhash
}

// var inputs [6]common.Hash
var outputs [2]common.Hash

// 	inputs[1] = blockHeader.TxHash
//	inputs[2] = blockHeader.Coinbase.Hash()
//	inputs[3] = blockHeader.UncleHash
//	inputs[4] = common.BigToHash(big.NewInt(int64(blockHeader.GasLimit)))
//	inputs[5] = common.BigToHash(big.NewInt(int64(blockHeader.Time)))

var inputs Inputs

type Inputs struct {
	ParentHash common.Hash
	TxHash     common.Hash
	Coinbase   common.Address
	UncleHash  common.Hash
	GasLimit   uint64
	Time       uint64

	Signer common.Address
}

func (i Inputs) Bytes() []byte {
	bb := bytes.Buffer{}
	enc := gob.NewEncoder(&bb)
	enc.Encode(i)
	return bb.Bytes()
}

func (i *Inputs) FromBytes(inputBytes []byte) {
	bb := bytes.NewReader(inputBytes)
	dec := gob.NewDecoder(bb)
	dec.Decode(i)
}

func Output(output common.Hash, receipts common.Hash) {
	if receipts != outputs[1] {
		fmt.Println("WARNING, receipts don't match", receipts, "!=", outputs[1])
		// panic("BAD receipts")
	}
	if output == outputs[0] {
		fmt.Println("good transition")
	} else {
		fmt.Println(output, "!=", outputs[0])
		// panic("BAD transition :((")
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func prefetchUncles(blockHash common.Hash, uncleHash common.Hash, hasher types.TrieHasher) {
	jr := jsonrespi{}
	{
		r := jsonreq{Jsonrpc: "2.0", Method: "eth_getUncleCountByBlockHash", Id: 1}
		r.Params = make([]interface{}, 1)
		r.Params[0] = blockHash.Hex()
		jsonData, _ := json.Marshal(r)
		check(json.NewDecoder(getAPI(jsonData)).Decode(&jr))
	}

	var uncles []*types.Header
	for u := 0; u < int(jr.Result); u++ {
		jr2 := jsonrespt{}
		{
			r := jsonreq{Jsonrpc: "2.0", Method: "eth_getUncleByBlockHashAndIndex", Id: 1}
			r.Params = make([]interface{}, 2)
			r.Params[0] = blockHash.Hex()
			r.Params[1] = fmt.Sprintf("0x%x", u)
			jsonData, _ := json.Marshal(r)

			/*a, _ := ioutil.ReadAll(getAPI(jsonData))
			fmt.Println(string(a))*/

			check(json.NewDecoder(getAPI(jsonData)).Decode(&jr2))
		}
		uncleHeader := jr2.Result.ToHeader()
		uncles = append(uncles, &uncleHeader)
		//fmt.Println(uncleHeader)
		//fmt.Println(jr2.Result)
	}

	unclesRlp, _ := rlp.EncodeToBytes(uncles)
	hash := crypto.Keccak256Hash(unclesRlp)

	if hash != uncleHash {
		panic("wrong uncle hash")
	}

	preimages[hash] = unclesRlp
}

func GetBlockNumber() *big.Int {
	r := jsonreq{Jsonrpc: "2.0", Method: "eth_blockNumber", Id: 1}
	jsonData, err := json.Marshal(r)
	check(err)

	jr := jsonresps{}
	check(json.NewDecoder(getAPI(jsonData)).Decode(&jr))
	ret := big.NewInt(0)
	ret, _ = ret.SetString(jr.Result[2:], 16)
	return ret
}

func GetBlockHeader(blockNumber *big.Int) types.Header {
	r := jsonreq{Jsonrpc: "2.0", Method: "eth_getBlockByNumber", Id: 1}
	r.Params = make([]interface{}, 2)
	r.Params[0] = fmt.Sprintf("0x%x", blockNumber.Int64())
	r.Params[1] = true
	jsonData, err := json.Marshal(r)
	check(err)

	jr := jsonrespt{}
	check(json.NewDecoder(getAPI(jsonData)).Decode(&jr))
	//fmt.Println(jr.Result)
	return jr.Result.ToHeader()
}

var extraSeal = 65

func encodeSigHeader(w io.Writer, header *types.Header) {
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-65], // Yes, this will panic if extra is too short
		header.MixDigest,
		header.Nonce,
	}

	if params.BorMainnetChainConfig.Bor.IsJaipur(header.Number.Uint64()) {
		if header.BaseFee != nil {
			enc = append(enc, header.BaseFee)
		}
	}

	if err := rlp.Encode(w, enc); err != nil {
		panic("can't encode: " + err.Error())
	}
}

func SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.Sum(hash[:0])

	return hash
}

func BorEcrecover(header *types.Header) (common.Address, error) {
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return common.Address{}, fmt.Errorf("errMissingSignature")
	}

	signature := header.Extra[len(header.Extra)-extraSeal:]

	// Recover the public key and the Ethereum address
	pubkey, err := crypto.Ecrecover(SealHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}

	var signer common.Address

	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	return signer, nil
}

func PrefetchBlock(blockNumber *big.Int, startBlock bool, hasher types.TrieHasher) {
	r := jsonreq{Jsonrpc: "2.0", Method: "eth_getBlockByNumber", Id: 1}
	r.Params = make([]interface{}, 2)
	r.Params[0] = fmt.Sprintf("0x%x", blockNumber.Int64())
	r.Params[1] = true
	jsonData, err := json.Marshal(r)
	check(err)

	jr := jsonrespt{}
	check(json.NewDecoder(getAPI(jsonData)).Decode(&jr))
	//fmt.Println(jr.Result)
	blockHeader := jr.Result.ToHeader()

	// put in the start block header
	if startBlock {
		blockHeaderRlp, err := rlp.EncodeToBytes(&blockHeader)
		check(err)
		hash := crypto.Keccak256Hash(blockHeaderRlp)
		preimages[hash] = blockHeaderRlp
		emptyHash := common.Hash{}
		if inputs.ParentHash == emptyHash {
			inputs.ParentHash = hash
		}
		return
	}

	// second block
	if blockHeader.ParentHash != inputs.ParentHash {
		fmt.Println(blockHeader.ParentHash, inputs.ParentHash)
		panic("block transition isn't correct")
	}
	inputs.TxHash = blockHeader.TxHash
	inputs.Coinbase = blockHeader.Coinbase
	inputs.UncleHash = blockHeader.UncleHash
	inputs.GasLimit = blockHeader.GasLimit
	inputs.Time = blockHeader.Time

	if inputs.Signer, err = BorEcrecover(&blockHeader); err != nil {
		panic(err)
	}
	println(fmt.Sprintf("block signer: %v", inputs.Signer.String()))

	// save the inputs
	saveinput := inputs.Bytes()
	inputhash = crypto.Keccak256Hash(saveinput)
	preimages[inputhash] = saveinput
	ioutil.WriteFile(fmt.Sprintf("%s/input", root), inputhash.Bytes(), 0644)

	// secret input aka output
	outputs[0] = blockHeader.Root
	outputs[1] = blockHeader.ReceiptHash

	// save the outputs
	saveoutput := make([]byte, 0)
	for i := 0; i < len(outputs); i++ {
		saveoutput = append(saveoutput, outputs[i].Bytes()[:]...)
	}
	ioutil.WriteFile(fmt.Sprintf("%s/output", root), saveoutput, 0644)

	// save the txs
	txs := make([]*types.Transaction, len(jr.Result.Transactions))
	for i := 0; i < len(jr.Result.Transactions); i++ {
		txs[i] = jr.Result.Transactions[i].ToTransaction()
	}
	testTxHash := types.DeriveSha(types.Transactions(txs), hasher)
	if testTxHash != blockHeader.TxHash {
		fmt.Println(testTxHash, "!=", blockHeader.TxHash)
		// panic("tx hash derived wrong")
		println("WARNING: tx hash derived wrong???")
	}

	// save the uncles
	prefetchUncles(blockHeader.Hash(), blockHeader.UncleHash, hasher)
}

func GetProofAccount(blockNumber *big.Int, addr common.Address, skey common.Hash, storage bool) []string {
	addrHash := crypto.Keccak256Hash(addr[:])
	unhashMap[addrHash] = addr

	r := jsonreq{Jsonrpc: "2.0", Method: "eth_getProof", Id: 1}
	r.Params = make([]interface{}, 3)
	r.Params[0] = addr
	r.Params[1] = [1]common.Hash{skey}
	r.Params[2] = fmt.Sprintf("0x%x", blockNumber.Int64())
	jsonData, _ := json.Marshal(r)
	jr := jsonresp{}
	json.NewDecoder(getAPI(jsonData)).Decode(&jr)

	if storage {
		return jr.Result.StorageProof[0].Proof
	} else {
		return jr.Result.AccountProof
	}
}

func getProvedCodeBytes(blockNumber *big.Int, addrHash common.Hash) []byte {
	addr := unhash(addrHash)

	r := jsonreq{Jsonrpc: "2.0", Method: "eth_getCode", Id: 1}
	r.Params = make([]interface{}, 2)
	r.Params[0] = addr
	r.Params[1] = fmt.Sprintf("0x%x", blockNumber.Int64())
	jsonData, _ := json.Marshal(r)
	jr := jsonresps{}
	json.NewDecoder(getAPI(jsonData)).Decode(&jr)

	//fmt.Println(jr.Result)

	// curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getCode","params":["0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b", "0x2"],"id":1}'

	ret, _ := hex.DecodeString(jr.Result[2:])
	//fmt.Println(ret)
	return ret
}

func getRawStorage(storageHash common.Hash) []byte {
	r := jsonreq{Jsonrpc: "2.0", Method: "bor_getRawStorage", Id: 1}
	r.Params = make([]interface{}, 1)
	r.Params[0] = storageHash.Hex()
	jsonData, _ := json.Marshal(r)
	jr := jsonresps{}
	json.NewDecoder(getAPI(jsonData)).Decode(&jr)
	ret, _ := hex.DecodeString(jr.Result[2:])
	return ret
}
