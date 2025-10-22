// Copyright 2023 Flant JSC
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

package hugo

import (
	"sync/atomic"
	"testing"

	"github.com/bep/lazycache"
	"github.com/gohugoio/hugo/common/loggers"
	"github.com/gohugoio/hugo/hugolib"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestHugFromConfig_ErrorHandling(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("HugFromConfig panicked when it should have returned an error: %v", r)
		}
	}()

	cmd := &command{
		configVersionID: atomic.Int32{},
		hugoSites: lazycache.New(lazycache.Options[int32, *hugolib.HugoSites]{
			MaxEntries: 1,
		}),
		flags:      &Flags{},
		logger:     log.NewNop(),
		hugologger: loggers.NewDefault(),
	}

	// Invalid commonConfig that will cause NewHugoSites to fail
	conf := &commonConfig{
		configs: nil, // This will cause hugolib.NewHugoSites to return an error
		fs:      nil,
	}

	result, err := cmd.HugFromConfig(conf)

	// Error should be returned instead of panic
	if err == nil {
		t.Error("expected an error to be returned when configs are nil, but got nil error")
	}

	// Verify that the result is nil when there's an error
	if result != nil {
		t.Error("expected result to be nil when an error occurs, but got non-nil result")
	}
}
