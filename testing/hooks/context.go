package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"

	"github.com/deckhouse/deckhouse/testing/library"
)

type BindingContextsSlice struct {
	JSON string
}

func (bcs *BindingContextsSlice) Set(contexts ...string) {
	if len(contexts) == 0 {
		bcs.JSON = `""`
		return
	}

	var rawContexts []interface{}
	for _, jsonContext := range contexts {
		var data []interface{}

		err := json.Unmarshal([]byte(jsonContext), &data)
		if err != nil {
			// TODO: Remove panic here
			panic(err)
		}
		for _, context := range data {
			rawContexts = append(rawContexts, context)
		}
	}

	combinedContexts, err := json.Marshal(rawContexts)
	if err != nil {
		// TODO: Remove panic here
		panic(err)
	}
	bcs.JSON = string(combinedContexts)
}

func (bcs *BindingContextsSlice) Get(path string) library.KubeResult {
	return library.KubeResult{Result: gjson.Get(bcs.JSON, path)}
}

func (bcs *BindingContextsSlice) Parse() library.KubeResult {
	return library.KubeResult{Result: gjson.Parse(bcs.JSON)}
}

func (bcs *BindingContextsSlice) Array() []gjson.Result {
	return library.KubeResult{Result: gjson.Parse(bcs.JSON)}.Array()
}

func (bcs *BindingContextsSlice) String() string {
	return bcs.JSON
}

var (
	OnStartupContext  = `[{"binding":"onStartup"}]`
	BeforeHelmContext = `[{"binding":"beforeHelm"}]`
	AfterHelmContext  = `[{"binding":"afterHelm"}]`
)

func ScheduleBindingContext(name string) string {
	return fmt.Sprintf(`[{"binding":"schedule","name":%q}]`, name)
}
