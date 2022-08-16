package analysis

import (
	"fmt"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/heimdalr/dag"
	"sort"
	"strconv"
	"strings"
)

type DAG struct {
	*dag.DAG
}

func BuildDAG(deps *state.StateDBDeps) (d DAG) {
	d = DAG{dag.NewDAG()}
	d.AddVertex(deps.TxDeps[0]) // make sure 0 is added ...
	for i := len(deps.TxDeps) - 1; i > 0; i-- {
		txTo := deps.TxDeps[i]
		txToId, _ := d.AddVertex(txTo)
		for j := i - 1; j >= 0; j-- {
			txFrom := deps.TxDeps[j]
			if txFrom.HasReadDep(txTo) {
				txFromId, _ := d.AddVertex(txFrom)
				d.AddEdge(txFromId, txToId)
				break // once we add a 'backward' dep we can't execute before that transaction so no need to proceed
			}
		}
	}
	return
}

func (d DAG) Report(out func(string)) {
	mustAtoI := func(s string) int {
		if i, err := strconv.Atoi(s); err != nil {
			panic(err)
		} else {
			return i
		}
	}

	var roots []int
	for k, _ := range d.GetRoots() {
		roots = append(roots, mustAtoI(k))
	}
	sort.Ints(roots)

	makeStrs := func(ints []int) (ret []string) {
		for _, v := range ints {
			ret = append(ret, fmt.Sprint(v))
		}
		return
	}

	maxDesc := 0
	maxDeps := 0
	totalDeps := 0
	for _, v := range roots {
		r, _ := d.GetVertex(fmt.Sprint(v))

		cntDeps := r.(*state.TxDeps).CntDeps()

		ids := []int{v}
		desc, _ := d.GetDescendants(fmt.Sprint(v))
		for kd, kv := range desc {
			ids = append(ids, mustAtoI(kd))
			cntDeps += kv.(*state.TxDeps).CntDeps()
		}
		sort.Ints(ids)
		out(fmt.Sprintf("(%v, %v) %v", len(ids), cntDeps, strings.Join(makeStrs(ids), "->")))

		if len(desc) > maxDesc {
			maxDesc = len(desc)
		}
		if cntDeps > maxDeps {
			maxDeps = cntDeps
		}
		totalDeps += cntDeps
	}

	numTx := len(d.DAG.GetVertices())
	out(fmt.Sprintf("max chain length: %v of %v (%v%%)", maxDesc+1, numTx,
		fmt.Sprintf("%.1f", float64(maxDesc+1)*100.0/float64(numTx))))
	out(fmt.Sprintf("max dep count: %v of %v (%v%%)", maxDeps, totalDeps,
		fmt.Sprintf("%.1f", float64(maxDeps)*100.0/float64(totalDeps))))
}
