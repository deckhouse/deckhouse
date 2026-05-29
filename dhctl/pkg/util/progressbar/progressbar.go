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

package progressbar

import (
	"fmt"
	"math"
	"time"

	"github.com/pterm/pterm"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

var defaultpb *Pb

func GetDefaultPb() *Pb {
	return defaultpb
}

type PbParam struct {
	startMsg   string
	size       int
	labelChan  chan string
	phasesChan chan phases.Progress
}

func NewPbParams(size int, startMsg string, labelChan chan string, phasesChan chan phases.Progress) *PbParam {
	return &PbParam{
		size:       size,
		startMsg:   startMsg,
		labelChan:  labelChan,
		phasesChan: phasesChan,
	}
}

type Pb struct {
	ProgressBarPrinter *pterm.ProgressbarPrinter
	MultiPrinter       *pterm.MultiPrinter
	SpinnerPrinter     *pterm.SpinnerPrinter
	StopCh             chan struct{}
}

func InitProgressBarWithDeferredFunc(name string, logger log.Logger) (func(), chan phases.Progress, error) {
	intLogger, ok := logger.(*log.InteractiveLogger)
	if !ok {
		return nil, nil, fmt.Errorf("logger is not interactive")
	}
	labelChan := intLogger.GetPhaseChan()
	phasesChan := make(chan phases.Progress, 5)

	pbParam := NewPbParams(100, name, labelChan, phasesChan)

	if err := InitProgressBar(pbParam); err != nil {
		return nil, phasesChan, err
	}

	onComplete := func() {
		FinishDefaultProgressBar()
	}

	return onComplete, phasesChan, nil
}

func InitProgressBar(param *PbParam) error {
	multi := pterm.DefaultMultiPrinter
	if param.size == 0 {
		param.size = 100
	}
	p := pterm.DefaultProgressbar.
		WithTotal(param.size).
		WithMaxWidth(120).
		WithWriter(multi.NewWriter()).
		WithTitle(param.startMsg)

	staticSpinner := pterm.DefaultSpinner.
		WithSequence(" ").
		WithDelay(time.Hour).
		WithWriter(multi.NewWriter())

	_, startErr := multi.Start()
	if startErr != nil {
		return startErr
	}

	var err error
	p, err = p.Start(param.startMsg)
	if err != nil {
		return err
	}
	_, err = staticSpinner.Start("Current action: ")
	if err != nil {
		return err
	}

	stopChan := make(chan struct{}, 2)

	defaultpb = &Pb{
		ProgressBarPrinter: p,
		MultiPrinter:       &multi,
		SpinnerPrinter:     staticSpinner,
		StopCh:             stopChan,
	}

	log.WithProgressBar()

	go updateProgress(p, param.labelChan, param.phasesChan, stopChan, staticSpinner, &multi)

	return nil
}

func updateProgress(
	p *pterm.ProgressbarPrinter,
	labelChan chan string,
	successChan chan phases.Progress,
	stopChan chan struct{},
	spinner *pterm.SpinnerPrinter,
	mp *pterm.MultiPrinter,
) {
	if p == nil {
		return
	}

	if !p.IsActive {
		return
	}

	inc := 0
	lastCompleted := ""

	for {
		select {
		case <-stopChan:
			return
		case msg, ok := <-labelChan:
			if !ok {
				return
			}
			spinner.UpdateText(pterm.Sprintf("Current action: %s", replaceStatus(msg)))
		case msg, ok := <-successChan:
			if !ok {
				return
			}

			if inc == 0 || lastCompleted == "" {
				// calculate increment
				phasesCount := len(msg.Phases)
				for _, p := range msg.Phases {
					phasesCount += len(p.SubPhases)
				}
				inc = 100 / phasesCount

				text := phaseToString(msg, false)
				if text != "" {
					p.UpdateTitle(text)
				}
			}

			if msg.CompletedPhase != "" {
				completed := phaseToString(msg, true)

				if completed == lastCompleted {
					continue
				}

				pterm.Success.WithWriter(mp.NewWriter()).Println(completed)
				increment := int(math.Round(msg.Progress*100) - float64(p.Current))

				if increment == 0 {
					increment = inc
				}

				if p.Current+increment > 100 {
					increment = 100 - p.Current
				}

				p.Add(increment)
				lastCompleted = completed
				p.UpdateTitle(phaseToString(msg, false))
			}
		}
	}
}

// if Progressbar used, this func allows to print to new MultiPrinter Writer
func InfoF(format string, a ...any) {
	writer := defaultpb.MultiPrinter.NewWriter()
	pterm.Info.WithWriter(writer).Printf(format, a...)
}

func WarnF(format string, a ...any) {
	writer := defaultpb.MultiPrinter.NewWriter()
	pterm.Warning.WithWriter(writer).Printf(format, a...)
}

func ErrorF(format string, a ...any) {
	if defaultpb != nil {
		writer := defaultpb.MultiPrinter.NewWriter()
		pterm.Error.WithWriter(writer).Printf(format, a...)
	} else {
		pterm.Error.Printf(format, a...)
	}
}

func phaseToString(p phases.Progress, completed bool) string {
	// Butify bootstrap: phases with subphases
	phasesMap := make(map[phases.OperationPhase]string)
	phasesMap[phases.BaseInfraPhase] = "Base Infrastructure"
	phasesMap[phases.RegistryPackagesProxyPhase] = "Preparing registry packages proxy"
	phasesMap[phases.ExecuteBashibleBundlePhase] = "Bootstrap Kubernetes on first master node"
	phasesMap[phases.InstallDeckhousePhase] = "Install Deckhouse"
	phasesMap[phases.CreateResourcesPhase] = "Create resources"
	phasesMap[phases.InstallAdditionalMastersAndStaticNodes] = "Install additional master nodes and CloudPermanent nodes"
	phasesMap[phases.ExecPostBootstrapPhase] = "Execute post-bootstrap script"
	phasesMap[phases.DeleteResourcesPhase] = "Delete resources"
	phasesMap[phases.AllNodesPhase] = "Process all nodes"
	phasesMap[phases.FinalizationPhase] = "Finalization"

	phasesMap[phases.ConvergeCheckPhase] = "Check converge"
	phasesMap[phases.ScaleToMultiMasterPhase] = "Scale cluster to multimaster"
	phasesMap[phases.ScaleToSingleMasterPhase] = "Scale cluster to singlemaster"
	phasesMap[phases.DeckhouseConfigurationPhase] = "Configure Deckhouse"

	phasesMap[phases.CreateStaticDestroyerNodeUserPhase] = "Create NodeUser for static destroyer"
	phasesMap[phases.UpdateStaticDestroyerIPs] = "Update static destroyer IPs"
	phasesMap[phases.WaitStaticDestroyerNodeUserPhase] = "Wait for NodeUser"
	phasesMap[phases.SetDeckhouseResourcesDeletedPhase] = "Set Deckhouse resources to deleted"
	phasesMap[phases.CommanderUUIDWasChecked] = "Commander UUID was checked"

	subphasesMap := make(map[phases.OperationSubPhase]string)
	subphasesMap[phases.InstallDeckhouseSubPhaseConnect] = "Connect to master host"
	subphasesMap[phases.InstallDeckhouseSubPhaseInstall] = "Install..."
	subphasesMap[phases.InstallDeckhouseSubPhaseWait] = "Wait for the first master readiness"
	subphasesMap[phases.OperationSubPhase(phases.CheckInfra)] = "Check Infrastructure"
	subphasesMap[phases.OperationSubPhase(phases.CheckConfiguration)] = "Check configuration"

	msg := ""
	if completed {
		if p.CompletedSubPhase != "" {
			msg = fmt.Sprintf("%s: %s", phasesMap[p.CurrentPhase], subphasesMap[p.CompletedSubPhase])
		} else {
			msg = phasesMap[p.CompletedPhase]
		}
	} else {
		if p.CurrentSubPhase != "" {
			msg = fmt.Sprintf("%s: %s", phasesMap[p.CurrentPhase], subphasesMap[p.CurrentSubPhase])
		} else {
			if p.CurrentPhase != "" {
				msg = phasesMap[p.CurrentPhase]
			} else {
				msg = phasesMap[p.CompletedPhase]
			}
		}
	}

	return fmt.Sprintf("%-60s", msg)
}

func replaceStatus(msg string) string {
	var res string
	switch msg {
	case "NodeGroups status":
		res = "Waiting for NodeGroup readiness"
	case "Resource not ready":
		res = "Waiting for resources readiness"
	case "Resource ready":
		res = "Waiting for resources readiness"
	default:
		res = msg
	}

	return res
}

func FinishDefaultProgressBar() {
	pb := GetDefaultPb()
	if pb == nil {
		return
	}

	// stopping the updateProgress goroutine
	pb.StopCh <- struct{}{}
	time.Sleep(50 * time.Millisecond)

	pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
	if _, err := pb.MultiPrinter.Stop(); err != nil {
		log.WarnF("failed to stop multi printer: %v", err)
	}
}
