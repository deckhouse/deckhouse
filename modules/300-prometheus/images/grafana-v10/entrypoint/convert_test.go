/*
Copyright 2023 Flant JSC

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

package main

import (
	"os"
	"testing"
)

const (
	testEnv1          = "GF_TEST1__FILE"
	testEnv2          = "GF_TEST2__FILE"
	expectedEnv1      = "GF_TEST1"
	expectedEnv2      = "GF_TEST2"
	testdata          = "testdata/test.txt"
	expectedEnv1Value = "test\n"
	expectedEnv2Value = "test\n"
)

func TestConvertEnv(t *testing.T) {
	os.Setenv(testEnv1, testdata)
	os.Setenv(testEnv2, testdata)
	os.Unsetenv(expectedEnv1)
	os.Unsetenv(expectedEnv2)

	err := convertEnv()
	if err != nil {
		t.Error(err)
	}
	v, ok := os.LookupEnv(expectedEnv1)
	if !ok {
		t.Errorf("env1 fail")
	}
	if v != expectedEnv1Value {
		t.Errorf("env1 value fail")
	}

	v, ok = os.LookupEnv(expectedEnv2)
	if !ok {
		t.Errorf("env2 fail")
	}
	if v != expectedEnv2Value {
		t.Errorf("env2 value fail")
	}

	if _, ok = os.LookupEnv(testEnv1); ok {
		t.Errorf("old param %s is not empty", testEnv1)
	}
	if _, ok = os.LookupEnv(testEnv2); ok {
		t.Errorf("old param %s is not empty", testEnv2)
	}
}

func TestConvertEnvExpectedError(t *testing.T) {
	os.Setenv(testEnv1, expectedEnv1Value)
	os.Setenv(testEnv2, expectedEnv2Value)
	os.Setenv(expectedEnv1, expectedEnv1Value)
	os.Setenv(expectedEnv2, expectedEnv2Value)

	err := convertEnv()
	if err == nil {
		t.Errorf("exclusive error")
	}
}
