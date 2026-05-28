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

package parser

func init() {
	registerOPFunctions()
}

func registerOPFunctions() {
	Functions["op_defined"] = &Function{
		Name:       "op_defined",
		ArgTypes:   []ValueType{ValueTypeMatrix},
		ReturnType: ValueTypeVector,
	}
	Functions["op_replace_nan"] = &Function{
		Name:       "op_replace_nan",
		ArgTypes:   []ValueType{ValueTypeMatrix, ValueTypeScalar, ValueTypeScalar},
		Variadic:   1,
		ReturnType: ValueTypeVector,
	}
	Functions["op_smoothie"] = &Function{
		Name:       "op_smoothie",
		ArgTypes:   []ValueType{ValueTypeMatrix},
		ReturnType: ValueTypeVector,
	}
	Functions["op_zero_if_none"] = &Function{
		Name:       "op_zero_if_none",
		ArgTypes:   []ValueType{ValueTypeMatrix, ValueTypeScalar},
		Variadic:   1,
		ReturnType: ValueTypeVector,
	}
}
