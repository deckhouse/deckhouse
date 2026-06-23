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
	logChan    chan string
}

func NewPbParams(size int, startMsg string, labelChan chan string, phasesChan chan phases.Progress, logChan chan string) *PbParam {
	return &PbParam{
		size:       size,
		startMsg:   startMsg,
		labelChan:  labelChan,
		phasesChan: phasesChan,
		logChan:    logChan,
	}
}

type Pb struct {
	ProgressBarPrinter *pterm.ProgressbarPrinter
	MultiPrinter       *pterm.MultiPrinter
	SpinnerPrinter     *pterm.SpinnerPrinter
	StopCh             chan struct{}
	LogBox             *LogBox
	WriterFabric       *WriterFabric
}

func InitProgressBarWithDeferredFunc(name string, logger log.Logger) (func(), chan phases.Progress, error) {
	intLogger, ok := logger.(*log.InteractiveLogger)
	if !ok {
		return nil, nil, fmt.Errorf("logger is not interactive")
	}
	labelChan := intLogger.GetPhaseChan()
	phasesChan := make(chan phases.Progress, 5)
	logChan := intLogger.GetLogChan()

	pbParam := NewPbParams(100, name, labelChan, phasesChan, logChan)

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
	writerFabric := newWriterFabric(&multi)
	if param.size == 0 {
		param.size = 100
	}

	width := pterm.GetTerminalWidth()
	effectiveWidth := width - 10
	if width < 160 {
		effectiveWidth = 120
	}
	p := pterm.DefaultProgressbar.
		WithTotal(param.size).
		WithMaxWidth(effectiveWidth).
		WithWriter(writerFabric.GetWriter()).
		WithTitle(param.startMsg)

	staticSpinner := pterm.DefaultSpinner.
		WithSequence(" ").
		WithDelay(time.Hour).
		WithWriter(writerFabric.GetWriter())

	logBox := newLogBox(&writerFabric, param.logChan)

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

	err = logBox.Start()
	if err != nil {
		return err
	}

	stopChan := make(chan struct{}, 2)

	defaultpb = &Pb{
		ProgressBarPrinter: p,
		MultiPrinter:       &multi,
		SpinnerPrinter:     staticSpinner,
		StopCh:             stopChan,
		LogBox:             logBox,
		WriterFabric:       &writerFabric,
	}

	log.WithProgressBar(true)

	go updateProgress(p, param.labelChan, param.phasesChan, stopChan, staticSpinner, &writerFabric)
	go logBox.Update()

	return nil
}

func updateProgress(
	p *pterm.ProgressbarPrinter,
	labelChan chan string,
	successChan chan phases.Progress,
	stopChan chan struct{},
	spinner *pterm.SpinnerPrinter,
	writerFabric *WriterFabric,
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

				status := defaultpb.LogBox.getStatusString()
				if err := defaultpb.LogBox.Stop(); err != nil {
					return
				}

				pterm.Success.WithWriter(writerFabric.GetWriter()).Println(completed)
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

				logBox := newLogBox(writerFabric, defaultpb.LogBox.logChan).WithStatusString(status)
				if err := logBox.Start(); err != nil {
					return
				}
				defaultpb.LogBox = logBox
				go logBox.Update()
			}
		}
	}
}

// if Progressbar used, this func allows to print to new MultiPrinter Writer
func InfoF(format string, a ...any) {
	writer := defaultpb.LogBox.ShiftDown()
	pterm.Info.WithWriter(writer).Printf(format, a...)
}

func WarnF(format string, a ...any) {
	writer := defaultpb.LogBox.ShiftDown()
	pterm.Warning.WithWriter(writer).Printf(format, a...)
}

func ErrorF(format string, a ...any) {
	if defaultpb != nil {
		writer := defaultpb.WriterFabric.GetWriter()
		pterm.Error.WithWriter(writer).Printf(format, a...)
	} else {
		pterm.Error.Printf(format, a...)
	}
}

func phaseToString(p phases.Progress, completed bool) string {
	// Beautify bootstrap: phases with subphases
	phasesMap := make(map[phases.OperationPhase]string)
	phasesMap[phases.PreInfraPreflightsPhase] = "Common preflight checks"
	phasesMap[phases.PostInfraPreflightsPhase] = "Static and post-infra preflight checks"
	phasesMap[phases.BaseInfraPhase] = "Base Infrastructure"
	phasesMap[phases.InstallKubernetesPhase] = "Install Kubernetes on the first master node"
	phasesMap[phases.InstallRegistryPhase] = "Set up the registry cache on the first master node"
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
	subphasesMap[phases.BaseInfraSubPhaseBaseInfra] = "Base Infrastructure"
	subphasesMap[phases.BaseInfraSubPhaseFirstMaster] = "First master node"
	subphasesMap[phases.InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters] = "Install additional master nodes"
	subphasesMap[phases.InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes] = "Install additional static nodes"
	subphasesMap[phases.InstallAdditionalMastersAndStaticNodesSubPhaseWait] = "Wait for control plane manager become ready"
	subphasesMap[phases.InstallKubernetesSubPhaseBundlePreparation] = "Prepare bashible bundle"
	subphasesMap[phases.InstallKubernetesSubPhaseRegistryPackagesProxy] = "Prepare registry packages proxy"
	subphasesMap[phases.InstallKubernetesSubPhaseNodePreparation] = "Prepare node"
	subphasesMap[phases.InstallKubernetesSubPhaseExecuteBashibleBundle] = "Execute bashible bundle"

	// TODO: too complicated, has to be refactored
	msg := ""
	if completed {
		if p.CompletedSubPhase != "" {
			currentPhase, ok := phasesMap[p.CurrentPhase]
			if !ok {
				currentPhase = string(p.CurrentPhase)
			}
			subPhase, ok := subphasesMap[p.CompletedSubPhase]
			if !ok {
				subPhase = string(p.CompletedSubPhase)
			}
			msg = fmt.Sprintf("%s: %s", currentPhase, subPhase)
		} else {
			completedPhase, ok := phasesMap[p.CompletedPhase]
			if !ok {
				completedPhase = string(p.CompletedPhase)
			}
			msg = completedPhase
		}
	} else {
		if p.CurrentSubPhase != "" {
			currentPhase, ok := phasesMap[p.CurrentPhase]
			if !ok {
				currentPhase = string(p.CurrentPhase)
			}
			subPhase, ok := subphasesMap[p.CurrentSubPhase]
			if !ok {
				subPhase = string(p.CurrentSubPhase)
			}
			msg = fmt.Sprintf("%s: %s", currentPhase, subPhase)
		} else {
			if p.CurrentPhase != "" {
				phase, ok := phasesMap[p.CurrentPhase]
				if !ok {
					phase = string(p.CurrentPhase)
				}
				msg = phase
			} else {
				phase, ok := phasesMap[p.CompletedPhase]
				if !ok {
					phase = string(p.CompletedPhase)
				}
				msg = phase
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

	if err := pb.LogBox.Stop(); err != nil {
		log.WarnF("failed to stop log box: %s\n", err.Error())
	}

	pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
	if _, err := pb.MultiPrinter.Stop(); err != nil {
		log.WarnF("failed to stop multi printer: %v", err)
	}
}
