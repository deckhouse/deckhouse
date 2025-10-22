/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"encoding/json"
	"fmt"

	bindingcontext "github.com/flant/shell-operator/pkg/hook/binding_context"
	. "github.com/flant/shell-operator/pkg/hook/types"
	"github.com/flant/shell-operator/test/hook/context"
	"github.com/tidwall/gjson"

	"github.com/deckhouse/deckhouse/testing/library"
)

type BindingContextsSlice struct {
	JSON            string
	BindingContexts []bindingcontext.BindingContext
}

func (bcs *BindingContextsSlice) Set(contexts ...context.GeneratedBindingContexts) {
	bcs.JSON = `""`
	bcs.BindingContexts = make([]bindingcontext.BindingContext, 0)

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
	bc := bindingcontext.BindingContext{
		Binding: string(binding),
	}
	bc.Metadata.BindingType = binding

	return context.GeneratedBindingContexts{
		Rendered:        fmt.Sprintf(`[{"binding":"%s"}]`, string(binding)),
		BindingContexts: []bindingcontext.BindingContext{bc},
	}
}
