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

// Package phase splits a long-running dhctl operation (bootstrap, converge,
// destroy) into a sequence of named, independently testable steps.
//
// A Phase is one step. The State parameter is the operation-specific bag of
// inputs/outputs the phases mutate as they run; each operation declares its
// own State type and parameterises Phase/Runner with it.
//
// The package is deliberately minimal: it does not interact with the
// progress tracker (phases.PhasedExecutionContext). Operations that need
// commander-visible phase events emit them inside the relevant phase's
// Run body — Runner only handles sequencing and error reporting.
package phase

import "context"

// Phase is one step of a phased operation. Name is used in log messages and
// to wrap any error the phase returns ("<name>: <inner>"); pick a short
// kebab-case label.
//
// Run reads and mutates state in place — phases later in the pipeline see
// whatever the earlier phases wrote.
type Phase[State any] interface {
	Name() string
	Run(ctx context.Context, state State) error
}
