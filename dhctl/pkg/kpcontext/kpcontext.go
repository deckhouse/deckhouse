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
