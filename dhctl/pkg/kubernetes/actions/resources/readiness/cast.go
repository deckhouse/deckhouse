// Copyright 2025 Flant JSC
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

package readiness

import (
	"fmt"
	"reflect"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type castErrorFunc func(key, kind string) (bool, error)
type castResult[T any] struct {
	value T
	ok    bool

	err   error
	ready bool
}

func (r *castResult[T]) ReadyResult() (bool, error) {
	return r.ready, r.err
}

func returnCastError[T any](key string, errFunc castErrorFunc) castResult[T] {
	ready, err := errFunc(key, reflect.TypeFor[T]().String())

	return castResult[T]{
		ok:    false,
		err:   err,
		ready: ready,
	}
}

func castKey[T any](m map[string]any, key string, notFound castErrorFunc, castErr castErrorFunc) castResult[T] {
	raw, ok := m[key]
	if !ok {
		return returnCastError[T](key, notFound)
	}

	result, ok := raw.(T)
	if !ok {
		return returnCastError[T](key, castErr)
	}

	return castResult[T]{
		value: result,
		ok:    true,
	}
}

func castVal[T any](raw any, castErr castErrorFunc) castResult[T] {
	result, ok := raw.(T)
	if !ok {
		return returnCastError[T]("", castErr)
	}

	return castResult[T]{
		value: result,
		ok:    true,
	}
}

func notFoundFuncDebugLogNotReady(logger log.Logger, resourceName string) castErrorFunc {
	return func(key, _ string) (bool, error) {
		logger.LogDebugF("Resource %s is not ready, because key %s not found.\n", resourceName, key)
		return false, nil
	}
}

func notFoundFuncDebugLogReady(logger log.Logger, resourceName string) castErrorFunc {
	return func(key, _ string) (bool, error) {
		logger.LogDebugF("Resource %s is ready, because key %s not found.\n", resourceName, key)
		return true, nil
	}
}

func castErrorFuncForResource(resourceName, additionalMsg string) castErrorFunc {
	return func(key, kind string) (bool, error) {
		msg := "value"
		if key != "" {
			msg = fmt.Sprintf("key %s", key)
		}

		if additionalMsg != "" {
			msg = fmt.Sprintf("%s (%s)", msg, additionalMsg)
		}

		return false, fmt.Errorf("Cannot check resource %s readiness because cannot cast %s to %s", resourceName, msg, kind)
	}
}
