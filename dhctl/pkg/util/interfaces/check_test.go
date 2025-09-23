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

package interfaces

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type testInterface interface {
	Do()
}

type testInterfaceImpl struct{}

func (*testInterfaceImpl) Do() {
	fmt.Println("Call Do on testInterfaceImpl")
}

func TestIsNil(t *testing.T) {
	require.True(t, IsNil(nil))

	var ii *testInterfaceImpl
	require.True(t, IsNil(ii))

	var err error
	require.True(t, IsNil(err))

	assertCallFuncWithInterface := func(t *testing.T, actual testInterface) {
		require.True(t, IsNil(actual))
	}

	var v testInterface
	require.True(t, IsNil(v))
	assertCallFuncWithInterface(t, v)

	var m map[string]struct{}
	require.True(t, IsNil(m))

	m = nil
	require.True(t, IsNil(m))

	var s []string
	require.True(t, IsNil(s))

	s = nil
	require.True(t, IsNil(s))

	var f func()
	require.True(t, IsNil(f))

	f = nil
	require.True(t, IsNil(f))
}

func TestIsNotNil(t *testing.T) {
	require.False(t, IsNil(struct{}{}))
	require.False(t, IsNil(testInterfaceImpl{}))
	require.False(t, IsNil(&testInterfaceImpl{}))

	var err error
	err = fmt.Errorf("test error")
	require.False(t, IsNil(err))

	i := testInterfaceImpl{}
	require.False(t, IsNil(i))

	ii := &testInterfaceImpl{}
	require.False(t, IsNil(ii))

	assertCallFuncWithInterface := func(t *testing.T, actual testInterface) {
		require.False(t, IsNil(actual))
	}

	var v testInterface
	v = &testInterfaceImpl{}
	require.False(t, IsNil(v))
	assertCallFuncWithInterface(t, v)

	m := make(map[string]struct{})
	require.False(t, IsNil(m))

	s := make([]string, 0, 0)
	require.False(t, IsNil(s))

	f := func() {}
	require.False(t, IsNil(f))
}

func TestIsNilAfterSet(t *testing.T) {
	err := fmt.Errorf("test error")
	err = nil
	require.True(t, IsNil(err))

	ii := &testInterfaceImpl{}
	ii = nil
	require.True(t, IsNil(ii))

	assertCallFuncWithInterface := func(t *testing.T, actual testInterface) {
		require.True(t, IsNil(actual))
	}

	var v testInterface = &testInterfaceImpl{}
	v = nil
	require.True(t, IsNil(v))
	assertCallFuncWithInterface(t, v)

	m := make(map[string]struct{})
	m = nil
	require.True(t, IsNil(m))

	s := make([]string, 0, 0)
	s = nil
	require.True(t, IsNil(s))

	f := func() {}
	f = nil
	require.True(t, IsNil(f))
}
