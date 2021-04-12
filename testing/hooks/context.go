package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/test/hook/context"
	"github.com/tidwall/gjson"

	"github.com/deckhouse/deckhouse/testing/library"
)

type BindingContextsSlice struct {
	JSON            string
	BindingContexts []binding_context.BindingContext
}

func (bcs *BindingContextsSlice) Set(contexts ...context.GeneratedBindingContexts) {
	bcs.JSON = `""`
	bcs.BindingContexts = make([]binding_context.BindingContext, 0)

	if len(contexts) == 0 {
		return
	}

	var rawContexts []interface{}
	for _, bc := range contexts {
		var data []interface{}

		err := json.Unmarshal([]byte(bc.Rendered), &data)
		if err != nil {
			// TODO: Remove panic here
			panic(err)
		}
		rawContexts = append(rawContexts, data...)

		bcs.BindingContexts = append(bcs.BindingContexts, bc.BindingContexts...)
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

// SimpleBindingGeneratedBindingContext is a helper to create empty binding contexts for OnStartup/Schedule/AfterHelm/etc.
func SimpleBindingGeneratedBindingContext(binding BindingType) context.GeneratedBindingContexts {
	bc := binding_context.BindingContext{
		Binding: string(binding),
	}
	bc.Metadata.BindingType = binding

	return context.GeneratedBindingContexts{
		Rendered:        fmt.Sprintf(`[{"binding":"%s"}]`, string(binding)),
		BindingContexts: []binding_context.BindingContext{bc},
	}
}
