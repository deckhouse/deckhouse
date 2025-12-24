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

package json

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
)

type testMsg struct {
	Int int    `json:"int"`
	Str string `json:"str"`
}

var (
	testCorrectJsonMsg = json.RawMessage([]byte(`
{
	"int": 1,
    "str": "string"
}
`))
	testSimpleJsonMsg        = json.RawMessage([]byte(`"string"`))
	testIncorrectJsonMsg     = json.RawMessage([]byte("{"))
	testEmptyArrayJsonMsg    = json.RawMessage([]byte("[]"))
	testNotEmptyArrayJsonMsg = json.RawMessage([]byte(`["string"]`))
	testEmptyObjectJsonMsg   = json.RawMessage([]byte(`{}`))
)

func TestUnmarshalMessage(t *testing.T) {
	result, err := UnmarshalToFromMessage[testMsg](testCorrectJsonMsg)
	assertTestMsg(t, result, err)

	resultStr, err := UnmarshalToFromMessage[string](testSimpleJsonMsg)
	assertTestStr(t, resultStr, err)

	resultEmptyArr, err := UnmarshalToFromMessage[[]testMsg](testEmptyArrayJsonMsg)
	require.NoError(t, err)
	require.False(t, govalue.IsNil(resultEmptyArr))
	require.Len(t, *resultEmptyArr, 0)

	resultArrPointer, err := UnmarshalToFromMessage[[]string](testNotEmptyArrayJsonMsg)
	require.NoError(t, err)
	require.False(t, govalue.IsNil(resultArrPointer))

	resultArr := *resultArrPointer
	require.Len(t, resultArr, 1)
	require.Equal(t, resultArr[0], "string")

	resultEmptyObject, err := UnmarshalToFromMessage[testMsg](testEmptyObjectJsonMsg)
	require.NoError(t, err)
	require.False(t, govalue.IsNil(resultEmptyObject))
	require.Empty(t, resultEmptyObject.Int)
	require.Empty(t, resultEmptyObject.Str)

	result, err = UnmarshalToFromMessage[testMsg](testIncorrectJsonMsg)
	assertIncorrectTestMsg(t, result, err)

	resultIncorrectTyped, err := UnmarshalToFromMessage[testMsg](testNotEmptyArrayJsonMsg)
	require.Error(t, err)
	require.True(t, govalue.IsNil(resultIncorrectTyped))
}

func TestUnmarshalMessageMap(t *testing.T) {
	const (
		correct   = "testCorrectJsonMsg"
		simple    = "testSimpleJsonMsg"
		incorrect = "testIncorrectJsonMsg"
	)

	m := map[string]json.RawMessage{
		correct:   testCorrectJsonMsg,
		simple:    testSimpleJsonMsg,
		incorrect: testIncorrectJsonMsg,
	}

	initialLen := len(m)

	result, err := UnmarshalToFromMessageMap[testMsg](m, correct)
	assertTestMsg(t, result, err)
	require.Len(t, m, initialLen)

	resultStr, err := UnmarshalToFromMessageMap[string](m, simple)
	assertTestStr(t, resultStr, err)
	require.Len(t, m, initialLen)

	result, err = UnmarshalToFromMessageMap[testMsg](m, incorrect)
	assertIncorrectTestMsg(t, result, err)
	require.Len(t, m, initialLen)

	resultNotExists, err := UnmarshalToFromMessageMap[testMsg](m, "not exists")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
	require.True(t, govalue.IsNil(resultNotExists))
	require.Len(t, m, initialLen)
}

func assertTestMsg(t *testing.T, result *testMsg, err error) {
	t.Helper()

	require.NoError(t, err)
	require.False(t, govalue.IsNil(result))
	require.Equal(t, result.Int, 1)
	require.Equal(t, result.Str, "string")
}

func assertIncorrectTestMsg(t *testing.T, result *testMsg, err error) {
	t.Helper()

	require.Error(t, err)
	require.True(t, govalue.IsNil(result))
}

func assertTestStr(t *testing.T, resultStr *string, err error) {
	t.Helper()

	require.NoError(t, err)
	require.False(t, govalue.IsNil(resultStr))
	require.Equal(t, *resultStr, "string")
}
