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
