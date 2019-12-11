package hooks

import (
	"github.com/deckhouse/deckhouse/testing/library"
	. "github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/tidwall/gjson"
)

type BindingContext struct {
	Binding      string                 `json:"binding"`
	Name         string                 `json:"name,omitempty"`
	Type         string                 `json:"type,omitempty"`
	WatchEvent   string                 `json:"watchEvent,omitempty"`
	Object       KubeObject             `json:"object,omitempty"`
	Objects      []BindingContextObject `json:"objects,omitempty"`
	FilterResult FilterResult           `json:"filterResult,omitempty"`
}

type BindingContextObject struct {
	Object       KubeObject   `json:"object"`
	FilterResult FilterResult `json:"filterResult"`
}

type BindingContextsSlice []BindingContext

type FilterResult string

func (bcs *BindingContextsSlice) Add(contexts ...BindingContext) {
	*bcs = append(*bcs, contexts...)
}

func (bcs *BindingContextsSlice) Set(contexts ...BindingContext) {
	*bcs = contexts
}

func (fr FilterResult) Get(path string) library.KubeResult {
	return library.KubeResult{Result: gjson.Get(string(fr), path)}
}

func (fr FilterResult) Parse() library.KubeResult {
	return library.KubeResult{Result: gjson.Parse(string(fr))}
}

func (fr FilterResult) String() string {
	return string(fr)
}

var (
	OnStartupContext  = BindingContext{Binding: "onStartup"}
	BeforeHelmContext = BindingContext{Binding: "beforeHelm"}
	AfterHelmContext  = BindingContext{Binding: "afterHelm"}
)

func ScheduleBindingContext(name string) BindingContext {
	return BindingContext{Binding: "schedule", Name: name}
}
