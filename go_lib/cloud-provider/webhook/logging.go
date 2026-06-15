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
	"fmt"

	"github.com/alecthomas/kingpin"
	logsv1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

// LogConfig holds logging settings for a cloud-provider admission webhook.
type LogConfig struct {
	format    string
	verbosity uint
}

// DefaultLogConfig returns the default logging configuration.
func DefaultLogConfig() LogConfig {
	return LogConfig{
		format:    "text",
		verbosity: 0,
	}
}

// InitLogFlags registers logging flags on the given kingpin application.
func InitLogFlags(cmd *kingpin.Application, cfg *LogConfig) {
	if cfg == nil {
		return
	}

	cmd.Flag("logging-format", "Logging format (text or json)").
		Envar("LOGGING_FORMAT").
		Default(cfg.format).
		StringVar(&cfg.format)
	cmd.Flag("v", "Number for the log level verbosity").
		Envar("VERBOSITY").
		Default("0").
		UintVar(&cfg.verbosity)
}

// SetupLogger applies logging configuration and wires controller-runtime to klog.
func SetupLogger(cfg LogConfig) error {
	opts := logsv1.NewLoggingConfiguration()
	opts.Format = cfg.format
	opts.Verbosity = logsv1.VerbosityLevel(cfg.verbosity)

	if err := logsv1.ValidateAndApply(opts, nil); err != nil {
		return fmt.Errorf("apply log options: %w", err)
	}

	ctrl.SetLogger(klog.Background())

	return nil
}
