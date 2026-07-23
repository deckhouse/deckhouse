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
	"os"

	"github.com/spf13/pflag"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/deckhouse/pkg/log"

	cloudlogging "github.com/deckhouse/deckhouse/go_lib/cloud-provider/logging"
)

// LogConfig holds logging settings for a cloud-provider admission webhook.
type LogConfig struct {
	format string
	level  string
}

// DefaultLogConfig returns the default logging configuration.
func DefaultLogConfig() LogConfig {
	return LogConfig{
		format: "text",
		level:  log.LevelInfo.String(),
	}
}

// InitLogFlags registers logging flags on the given pflag FlagSet.
func InitLogFlags(fs *pflag.FlagSet, cfg *LogConfig) {
	if cfg == nil || fs == nil {
		return
	}

	if format, ok := os.LookupEnv("LOGGING_FORMAT"); ok {
		cfg.format = format
	}
	if level, ok := os.LookupEnv("LOGGING_LEVEL"); ok {
		cfg.level = level
	}

	fs.StringVar(&cfg.format, "logging-format", cfg.format, "Logging format (text or json)")
	fs.StringVar(&cfg.level, "logging-level", cfg.level, "Logging level")
}

// SetupLogger applies logging configuration and wires controller-runtime to pkg/log.
func SetupLogger(cfg LogConfig) error {
	var handlerType log.HandlerType

	switch cfg.format {
	case "json":
		handlerType = log.JSONHandlerType
	default:
		handlerType = log.TextHandlerType
	}

	logger := log.NewLogger(
		log.WithHandlerType(handlerType),
		log.WithLevel(log.LogLevelFromStr(cfg.level).Level()),
	)

	ctrl.SetLogger(cloudlogging.NewLogrAdapter(logger))

	return nil
}
