package state

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

type TxDeps struct {
	id        string
	readDeps  map[string]bool
	writeDeps map[string]bool
}

func (td *TxDeps) ID() string {
	return td.id
}

const BalancePath = 1
const NoncePath = 2
const CodePath = 3
const SuicidePath = 4

type StateDBDeps struct {
	TxIdx int

	TxDeps map[int]*TxDeps

	ignorePrefixes []string
}

func (s *StateDBDeps) Report(f func(out string)) {
	cntReadDeps := 0
	cntWriteDeps := 0
	for _, v := range s.TxDeps {
		cntReadDeps += len(v.readDeps)
		cntWriteDeps += len(v.writeDeps)
	}

	avgReadDeps := float64(cntReadDeps) / float64(len(s.TxDeps))
	avgWriteDeps := float64(cntWriteDeps) / float64(len(s.TxDeps))
	f(fmt.Sprintf("average deps: read %.2f, write %.2f", avgReadDeps, avgWriteDeps))
}

func (td *TxDeps) CntDeps() int {
	return len(td.readDeps) + len(td.writeDeps)
}

func (s *StateDBDeps) ForEach(putDep func(int, string, bool)) {
	for txId, txDeps := range s.TxDeps {
		for dep, _ := range txDeps.readDeps {
			putDep(txId, dep, false)
		}
		for dep, _ := range txDeps.writeDeps {
			putDep(txId, dep, true)
		}
	}
}

func (s *StateDBDeps) SetIgnores(addrs []common.Address) {
	for i := range addrs {
		s.ignorePrefixes = append(s.ignorePrefixes, strings.ToLower(addrs[i].String()[2:]))
	}
}

func (d TxDeps) HasReadDep(txFrom *TxDeps) bool {
	for k, _ := range d.readDeps {
		if txFrom.writeDeps[k] {
			return true
		}
	}
	return false
}

func (s *StateDBDeps) ensureDeps() {
	if s.TxDeps == nil {
		s.TxDeps = make(map[int]*TxDeps)
	}
	if _, ok := s.TxDeps[s.TxIdx]; !ok {
		s.TxDeps[s.TxIdx] = &TxDeps{
			id:        fmt.Sprint(s.TxIdx),
			readDeps:  make(map[string]bool),
			writeDeps: make(map[string]bool),
		}
	}
}

func (s *StateDBDeps) readDep(dep string) {
	for _, pre := range s.ignorePrefixes {
		if strings.HasPrefix(dep, pre) {
			return
		}
	}
	s.ensureDeps()
	if td, ok := s.TxDeps[s.TxIdx]; ok {
		td.readDeps[dep] = true
	}
}

func (s *StateDBDeps) writeDep(dep string) {
	for _, pre := range s.ignorePrefixes {
		if strings.HasPrefix(dep, pre) {
			return
		}
	}
	s.ensureDeps()
	if td, ok := s.TxDeps[s.TxIdx]; ok {
		td.writeDeps[dep] = true
	}
}

func (s *StateDBDeps) CreateAccount(addr common.Address) {
	s.writeDep(NewSubpathKey(addr, BalancePath).String())
}

func (s *StateDBDeps) SubBalance(addr common.Address, b *big.Int) {
	s.GetBalance(addr)
	s.writeDep(NewSubpathKey(addr, BalancePath).String())
}

func (s *StateDBDeps) AddBalance(addr common.Address, b *big.Int) {
	s.GetBalance(addr)
	s.writeDep(NewSubpathKey(addr, BalancePath).String())
}

func (s *StateDBDeps) GetBalance(addr common.Address) *big.Int {
	s.readDep(NewSubpathKey(addr, BalancePath).String())
	return nil
}

func (s *StateDBDeps) SetBalance(addr common.Address, amount *big.Int) {
	s.writeDep(NewSubpathKey(addr, BalancePath).String())
}

func (s *StateDBDeps) GetNonce(addr common.Address) uint64 {
	s.readDep(NewSubpathKey(addr, NoncePath).String())
	return 0
}

func (s *StateDBDeps) SetNonce(addr common.Address, u uint64) {
	s.writeDep(NewSubpathKey(addr, NoncePath).String())
}

func (s *StateDBDeps) GetCodeHash(addr common.Address) common.Hash {
	s.readDep(NewSubpathKey(addr, CodePath).String())
	return common.Hash{}
}

func (s *StateDBDeps) GetCode(addr common.Address) []byte {
	s.readDep(NewSubpathKey(addr, CodePath).String())
	return nil
}

func (s *StateDBDeps) SetCode(addr common.Address, bytes []byte) {
	s.writeDep(NewSubpathKey(addr, CodePath).String())
}

func (s *StateDBDeps) GetCodeSize(addr common.Address) int {
	s.readDep(NewSubpathKey(addr, CodePath).String())
	return 0
}

func (s *StateDBDeps) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	s.readDep(NewStateKey(addr, hash).String())
	return common.Hash{}
}

func (s *StateDBDeps) GetState(addr common.Address, hash common.Hash) common.Hash {
	s.readDep(NewStateKey(addr, hash).String())
	return common.Hash{}
}

func (s *StateDBDeps) SetState(addr common.Address, key common.Hash, value common.Hash) {
	s.writeDep(NewStateKey(addr, key).String())
}

func (s *StateDBDeps) Suicide(addr common.Address) bool {
	s.writeDep(NewSubpathKey(addr, SuicidePath).String())
	s.writeDep(NewSubpathKey(addr, BalancePath).String())
	return false
}

func (s *StateDBDeps) HasSuicided(addr common.Address) bool {
	s.readDep(NewSubpathKey(addr, SuicidePath).String())
	return false
}
