// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package boot_test

import (
	"time"

	et "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/enginetest"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// quiet bounds a "nothing happens" check: the engine reacts within milliseconds in the kit.
const quiet = 500 * time.Millisecond

// checkpoint is what the controller passes to the next Boot for desired, from the Report the
// projection persisted: status.currentVersion and status.lastAppliedConfiguration.
func checkpoint(k *et.Kit, desired model.Desired) model.BootPackage {
	r := k.Report(desired.Name)
	enabled := true

	bp := model.BootPackage{
		Name:     desired.Name,
		Kind:     desired.Kind,
		Version:  r.CurrentVersion,
		Source:   desired.Source,
		Embedded: desired.Embedded,
	}

	if desired.Kind == model.KindModule {
		bp.Intent = &enabled
	}

	if r.Current.AppliedValues != nil {
		bp.Settings = &model.Settings{Values: r.Current.AppliedValues}
	}

	return bp
}

// running waits until name runs version with its checkpoint written.
func running(k *et.Kit, name, version string) model.Report {
	return k.WaitReport(name, "running "+version, func(r model.Report) bool {
		return et.Running(r, version)
	})
}

// enabled restores the module intent the controller passes at boot.
func enabled(bp model.BootPackage) model.BootPackage {
	on := true
	bp.Intent = &on

	return bp
}

// settled waits until the kit's history collector has recorded name's current Report, so a later
// history entry can only come from a later write.
func settled(k *et.Kit, name string) {
	k.Wait("history of "+name+" settled", func() bool {
		h := k.History(name)
		return len(h) > 0 && et.Dump(h[len(h)-1]) == et.Dump(k.Report(name))
	})
}
