// Copyright 2026 Flant JSC
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

package kpcontext

import (
	"context"

	"gopkg.in/alecthomas/kingpin.v2"
)

func SetContextToAction(ctx context.Context) kingpin.Action {
	return func(c *kingpin.ParseContext) error {
		SetContextToParseContext(ctx, c)

		return nil
	}
}

func SetContextToParseContext(ctx context.Context, c *kingpin.ParseContext) {
	for _, el := range c.Elements {
		if _, ok := el.Clause.(context.Context); ok {
			el.Clause = ctx

			return
		}
	}

	c.Elements = append(c.Elements, &kingpin.ParseElement{Clause: ctx})
}

func ExtractContext(c *kingpin.ParseContext) context.Context {
	for _, el := range c.Elements {
		if ctx, ok := el.Clause.(context.Context); ok {
			return ctx
		}
	}

	// fallback to context.Background(),
	// can be helpful in cases when context is not set (e.g., not in dhctl code)
	return context.Background()
}
