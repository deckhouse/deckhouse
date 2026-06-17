/*
Copyright 2025 Flant JSC

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

package promql

import (
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/util/annotations"
)

func init() {
	registerOPFunctionsParser()
}

func registerOPFunctionsParser() {
	parser.Functions["op_defined"] = &parser.Function{
		Name:       "op_defined",
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix},
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls["op_defined"] = funcOPStub

	parser.Functions["op_replace_nan"] = &parser.Function{
		Name:       "op_replace_nan",
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix, parser.ValueTypeScalar, parser.ValueTypeScalar},
		Variadic:   1,
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls["op_replace_nan"] = funcOPStub

	parser.Functions["op_smoothie"] = &parser.Function{
		Name:       "op_smoothie",
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix},
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls["op_smoothie"] = funcOPStub

	parser.Functions["op_zero_if_none"] = &parser.Function{
		Name:       "op_zero_if_none",
		ArgTypes:   []parser.ValueType{parser.ValueTypeMatrix, parser.ValueTypeScalar},
		Variadic:   1,
		ReturnType: parser.ValueTypeVector,
	}
	FunctionCalls["op_zero_if_none"] = funcOPStub
}

func funcOPStub(vals []parser.Value, args parser.Expressions, enh *EvalNodeHelper) (Vector, annotations.Annotations) {
	return nil, nil
}
