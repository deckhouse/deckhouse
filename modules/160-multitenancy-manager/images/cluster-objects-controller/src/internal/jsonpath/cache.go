package jsonpath

import (
	"sync"

	"github.com/theory/jsonpath"
)

var _ Factory = &CachingFactory{}

type CachingFactory struct {
	cache  *sync.Map
	parser *jsonpath.Parser
}

func NewWithCache() *CachingFactory {
	return &CachingFactory{
		cache:  &sync.Map{},
		parser: jsonpath.NewParser(),
	}
}

func (c *CachingFactory) Path(expr string) (*jsonpath.Path, error) {
	p, found := c.cache.Load(expr)
	if found {
		return p.(*jsonpath.Path), nil
	}

	fieldPath, err := c.parser.Parse(expr)
	if err != nil {
		return nil, err
	}

	c.cache.Store(expr, fieldPath)
	return fieldPath, nil
}
