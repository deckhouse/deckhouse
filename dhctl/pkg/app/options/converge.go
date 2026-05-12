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

package options

import "time"

// ConvergeOptions covers the converge / converge-exporter / migration commands.
type ConvergeOptions struct {
	MetricsPath   string
	ListenAddress string
	CheckInterval time.Duration
	OutputFormat  string

	CheckHasTerraformStateBeforeMigrateToTofu bool
}

// NewConvergeOptions returns ConvergeOptions with the previous package-level defaults.
func NewConvergeOptions() ConvergeOptions {
	return ConvergeOptions{
		MetricsPath:   "/metrics",
		ListenAddress: ":9101",
		CheckInterval: time.Minute,
		OutputFormat:  "yaml",
	}
}

// AutoConvergeOptions configures the periodical auto-converge service.
type AutoConvergeOptions struct {
	ApplyInterval   time.Duration
	ListenAddress   string
	RunningNodeName string
}

// NewAutoConvergeOptions returns AutoConvergeOptions with defaults.
func NewAutoConvergeOptions() AutoConvergeOptions {
	return AutoConvergeOptions{
		ApplyInterval: 30 * time.Minute,
		ListenAddress: ":9101",
	}
}
