package phases

import "github.com/deckhouse/deckhouse/dhctl/pkg/state"

type DhctlState map[string][]byte

func ExtractDhctlState(stateCache state.Cache) (res DhctlState, err error) {
	err = stateCache.Iterate(func(k string, v []byte) error {
		if res == nil {
			res = make(map[string][]byte)
		}
		res[k] = v
		return nil
	})
	return
}

type OperationWithState struct {
	stateCache state.Cache
}

func NewOperationWithState(stateCache state.Cache) *OperationWithState {
	return &OperationWithState{
		stateCache: stateCache,
	}
}

func (op *OperationWithState) Init(stateCache state.Cache) {
	op.stateCache = stateCache
}

func (op *OperationWithState) GetCacheState() (DhctlState, error) {
	return ExtractDhctlState(op.stateCache)
}
