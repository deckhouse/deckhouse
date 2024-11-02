// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package context

import (
	"context"
)

type ctxKey string

const customKey ctxKey = "custom_key"
const stackTrace ctxKey = "stack_trace"

func SetCustomKeyContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, customKey, true)
}

func GetCustomKeyContext(ctx context.Context) bool {
	has, ok := ctx.Value(customKey).(bool)
	if !ok {
		return false
	}

	return has
}

func SetStackTraceContext(ctx context.Context, trace string) context.Context {
	return context.WithValue(ctx, stackTrace, trace)
}

func GetStackTraceContext(ctx context.Context) *string {
	trace, ok := ctx.Value(stackTrace).(string)
	if !ok {
		return nil
	}

	return &trace
}
