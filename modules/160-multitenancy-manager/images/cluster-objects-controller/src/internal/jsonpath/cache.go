/*
Copyright 2026 Flant JSC

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