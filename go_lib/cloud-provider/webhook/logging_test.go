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

	"github.com/spf13/pflag"
)

func TestInitLogFlagsNilConfig(t *testing.T) {
	t.Parallel()

	InitLogFlags(pflag.NewFlagSet("test", pflag.ExitOnError), nil)
}

func TestInitLogFlagsPopulatesConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultLogConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	InitLogFlags(fs, &cfg)

	if err := fs.Parse([]string{"--logging-format=json", "--logging-level=debug"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.format != "json" || cfg.level != "debug" {
		t.Fatalf("InitLogFlags() cfg = %#v, want format=json level=debug", cfg)
	}
}

func TestSetupLogger(t *testing.T) {
	t.Parallel()

	cfg := DefaultLogConfig()
	cfg.format = "text"
	cfg.level = "info"

	if err := SetupLogger(cfg); err != nil {
		t.Fatalf("SetupLogger() error = %v", err)
	}
}
