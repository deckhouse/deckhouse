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

package kptelemetry

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	ottrace "go.opentelemetry.io/otel/trace"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

// todo: dirty hack - we cannot clearly destroy span in kingpin, so we need to use global variable
var (
	commandMu   sync.Mutex
	commandSpan ottrace.Span
)

func StartCommand(c *kingpin.ParseContext) error {
	ctx := kpcontext.ExtractContext(c)

	if c.SelectedCommand == nil {
		return nil
	}
	cmdName := c.String()

	ctx, span := telemetry.StartSpan(
		ctx,
		cmdName,
		ottrace.WithAttributes(
			attribute.String("dhctl.command", cmdName),
			attribute.String("process.command_line", strings.Join(os.Args, " ")),
		),
	)

	commandMu.Lock()
	commandSpan = span
	commandMu.Unlock()

	kpcontext.SetContextToParseContext(ctx, c)

	return nil
}

func EndCommand(err error, errorCode int) {
	commandMu.Lock()
	if commandSpan != nil {
		if err != nil {
			commandSpan.SetStatus(codes.Error, err.Error())
		} else if errorCode != 0 {
			commandSpan.SetStatus(codes.Error, fmt.Sprintf("exit code %d", errorCode))
		} else {
			commandSpan.SetStatus(codes.Ok, "")
		}
		commandSpan.End()

		log.DebugF("TraceID: %s\n", commandSpan.SpanContext().TraceID().String())
	}
	commandMu.Unlock()
}
