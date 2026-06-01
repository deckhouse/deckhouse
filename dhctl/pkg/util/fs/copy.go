// Copyright 2021 Flant JSC
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

package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
)

func CreateFileBackup(ctx context.Context, fName string) {
	suffix := time.Now().Format("150405-000")

	// Make copies of intermediate states.
	outName := fmt.Sprintf("%s-%s", fName, suffix)
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("save to: %s", outName))

	in, err := os.Open(fName)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("open '%s': %v", fName, err))
		return
	}
	defer in.Close()

	out, err := os.Create(outName)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("create copy '%s': %v", outName, err))
		return
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("save copy: %v", err))
		return
	}
	_ = out.Close()
}
