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

package webhook

import (
	"testing"

	"github.com/alecthomas/kingpin"
)

func TestInitLogFlagsNilConfig(t *testing.T) {
	t.Parallel()

	InitLogFlags(kingpin.New("test", "test"), nil)
}

func TestInitLogFlagsPopulatesConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultLogConfig()
	app := kingpin.New("test", "test")
	InitLogFlags(app, &cfg)

	if _, err := app.Parse([]string{"--logging-format=json", "--v=2"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.options.Format != "json" || cfg.verbosity != 2 {
		t.Fatalf("InitLogFlags() cfg = %#v, want format=json verbosity=2", cfg)
	}
}

func TestSetupLogger(t *testing.T) {
	t.Parallel()

	cfg := DefaultLogConfig()
	cfg.options.Format = "text"
	cfg.verbosity = 0

	if err := SetupLogger(cfg); err != nil {
		t.Fatalf("SetupLogger() error = %v", err)
	}
}
