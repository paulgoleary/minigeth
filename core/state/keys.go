package state

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"strings"
)

const addressType = 1
const stateType = 2
const subpathType = 3

const KeyLength = common.AddressLength + common.HashLength + 2

// type Key [KeyLength]byte

type Key struct {
	keyType byte
	addr    common.Address
	hash    common.Hash
	subPath byte
}

func (k *Key) IsAddress() bool {
	return k.keyType == addressType
}

func (k *Key) IsState() bool {
	return k.keyType == stateType
}

func (k *Key) IsSubpath() bool {
	return k.keyType == subpathType
}

func (k *Key) GetAddress() common.Address {
	return k.addr
}

func (k *Key) GetStateKey() common.Hash {
	return k.hash
}

func (k *Key) GetSubpath() byte {
	return k.subPath
}

func newKey(addr common.Address, hash common.Hash, subpath byte, keyType byte) Key {
	var k Key
	k.addr = addr
	k.hash = hash
	k.subPath = subpath
	k.keyType = keyType
	return k
}

func (k Key) String() string {
	switch k.keyType {
	case addressType:
		{
			return k.addr.String()
		}
	case stateType:
		{
			return fmt.Sprintf("%v:%v", strings.ToLower(k.addr.String()[2:]), strings.ToLower(k.hash.String()[2:]))
		}
	case subpathType:
		{
			return fmt.Sprintf("%v:%v", strings.ToLower(k.addr.String()[2:]), k.subPath)
		}
	default:
		panic(fmt.Errorf("should not happen - undefined type"))
	}
}

func NewAddressKey(addr common.Address) Key {
	return newKey(addr, common.Hash{}, 0, addressType)
}

func NewStateKey(addr common.Address, hash common.Hash) Key {
	k := newKey(addr, hash, 0, stateType)
	if !k.IsState() {
		panic(fmt.Errorf("key is not a state key"))
	}

	return k
}

var NilAddress = common.Address{}

func NewSubpathKey(addr common.Address, subpath byte) Key {
	return newKey(addr, common.Hash{}, subpath, subpathType)
}
