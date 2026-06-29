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
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/pterm/pterm"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

var (
	defaultpb                  *Pb
	ErrTerminalScreenIsToSmall = errors.New("Terminal screen has not enouth height")
)

func GetDefaultPb() *Pb {
	return defaultpb
}

type PbParam struct {
	startMsg     string
	size         int
	labelChan    chan string
	phasesChan   chan phases.Progress
	logChan      chan string
	phasesNumber int
}

func NewPbParams(size int, startMsg string, labelChan chan string, phasesChan chan phases.Progress, logChan chan string, phasesNumber int) *PbParam {
	return &PbParam{
		size:         size,
		startMsg:     startMsg,
		labelChan:    labelChan,
		phasesChan:   phasesChan,
		logChan:      logChan,
		phasesNumber: phasesNumber,
	}
}

type Pb struct {
	ProgressBarPrinter *pterm.ProgressbarPrinter
	MultiPrinter       *pterm.MultiPrinter
	SpinnerPrinter     *pterm.SpinnerPrinter
	StopCh             chan struct{}
	LogBox             *LogBox
	WriterFabric       *WriterFabric
	availableHeight    int
	needSupressSuccess bool
	successedPhases    []holdedMessage

	// pbOpts are using for control of appearance of LogBox
	pbOpts *pbOpts
}

// disable appearance of LogBox for additional masters subphase
func (p *Pb) WithEmptyAdditionalMasters() {
	if p == nil {
		return
	}

	p.pbOpts.aditionalMasters = false
}

// disable appearance of LogBox for additional static nodes subphase
func (p *Pb) WithEmptyAdditionalNGs() {
	if p == nil {
		return
	}

	p.pbOpts.staticNodes = false
}

// disable appearance of LogBox for additional static nodes subphase
func (p *Pb) WithEmptyBostBootstrapScript() {
	if p == nil {
		return
	}

	p.pbOpts.postBootstrapScripts = false
}

// disable appearance of LogBox for additional static nodes subphase
func (p *Pb) WithEmptyFinalization() {
	if p == nil {
		return
	}

	p.pbOpts.finalizationResources = false
}

type pbOpts struct {
	aditionalMasters      bool
	staticNodes           bool
	postBootstrapScripts  bool
	finalizationResources bool
}

func (p *Pb) CheckPhase(phase string, subphase ...string) bool {
	switch phase {
	case string(phases.InstallAdditionalMastersAndStaticNodes):
		switch subphase[0] {
		case string(phases.InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters):
			return p.pbOpts.aditionalMasters
		case string(phases.InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes):
			return p.pbOpts.staticNodes
		case string(phases.InstallAdditionalMastersAndStaticNodesSubPhaseWait):
			return false
		default:
			return true
		}
	case string(phases.ExecPostBootstrapPhase):
		return p.pbOpts.postBootstrapScripts
	case string(phases.FinalizationPhase):
		return p.pbOpts.finalizationResources
	default:
		return true
	}
}

func (p *Pb) PrintSupressedSuccess(msg string) {
	p.successedPhases = append(p.successedPhases, holdedMessage{mType: successMsgType, message: msg})
	if len(p.successedPhases) < 4 {
		pterm.Success.WithWriter(p.WriterFabric.GetWriter()).Println(msg)
	} else {
		last := p.successedPhases[(len(p.successedPhases) - 4):]
		for i, m := range last {
			oldWriter := p.MultiPrinter.Writer
			pterm.SetDefaultOutput(p.WriterFabric.allWriters[i+2])
			fmt.Print("\033[2K")
			pterm.Success.WithWriter(p.WriterFabric.allWriters[i+2]).Println(m.message)
			pterm.SetDefaultOutput(oldWriter)
		}
	}
}

func (p *Pb) FinalizeSuccess() {
	if p == nil {
		return
	}

	if !p.needSupressSuccess || len(p.successedPhases) == 0 {
		return
	}

	for i, m := range p.successedPhases {
		oldWriter := p.MultiPrinter.Writer
		var writer io.Writer
		if len(p.WriterFabric.allWriters) > i+2 {
			writer = p.WriterFabric.allWriters[i+2]
		} else {
			writer = p.WriterFabric.GetWriter()
		}
		pterm.SetDefaultOutput(writer)
		fmt.Print("\033[2K")
		if m.mType == successMsgType {
			pterm.Success.WithWriter(writer).Println(m.message)
		} else {
			pterm.Info.WithWriter(writer).Print(m.message)
		}

		pterm.SetDefaultOutput(oldWriter)
	}
	p.needSupressSuccess = false
	p.successedPhases = nil
}

type msgType string

const (
	successMsgType msgType = "success"
	infoMsgType    msgType = "info"
)

type holdedMessage struct {
	mType   msgType
	message string
}

func countPhases(phases []phases.PhaseWithSubPhases) int {
	count := 0
	for _, p := range phases {
		count++
		count += len(p.SubPhases)
	}

	return count
}

func InitProgressBarWithDeferredFunc(name string, logger log.Logger, phasesWithSubphases []phases.PhaseWithSubPhases) (func(), chan phases.Progress, error) {
	intLogger, ok := logger.(*log.InteractiveLogger)
	if !ok {
		return nil, nil, fmt.Errorf("logger is not interactive")
	}
	labelChan := intLogger.GetPhaseChan()
	phasesChan := make(chan phases.Progress, 5)
	logChan := intLogger.GetLogChan()

	pbParam := NewPbParams(100, name, labelChan, phasesChan, logChan, countPhases(phasesWithSubphases))

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
	effectiveWidth := width - 20
	if width < 160 {
		effectiveWidth = 120
	}

	// estimating height
	height := pterm.GetTerminalHeight()
	log.DebugF("estimated terminal width: %d, height: %d, effective height: %d\n", width, height, height-13)

	if param.phasesNumber > 10 {
		// bootstrap, at leaste 2 additional rows is needed, for info of master nodes IP
		param.phasesNumber += 2
	}

	supressSuccess := height-13-param.phasesNumber <= 0

	if height-param.phasesNumber <= 0 {
		// terminal screen is smaller then MultiPrinter output w/o LogBox, returning an error
		return ErrTerminalScreenIsToSmall
	}

	p := pterm.DefaultProgressbar.
		WithTotal(param.size).
		WithMaxWidth(effectiveWidth).
		// WithMaxWidth(120).
		WithWriter(writerFabric.GetWriter()).
		WithTitle(param.startMsg)

	staticSpinner := pterm.DefaultSpinner.
		WithSequence(" ").
		WithDelay(time.Hour).
		WithWriter(writerFabric.GetWriter())

	logBox := newLogBox(&writerFabric, param.logChan, 10)

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
		availableHeight:    height - 13, // 1 for pb, 1 for staticSpinner, 11 for logbox
		needSupressSuccess: supressSuccess,
		pbOpts: &pbOpts{
			aditionalMasters:      true,
			staticNodes:           true,
			postBootstrapScripts:  true,
			finalizationResources: true,
		},
	}

	log.WithProgressBar(true)
	// log.WithLogSending(true)

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

				if err := defaultpb.LogBox.Stop(); err != nil {
					return
				}

				// check out if suppress success is needed
				if defaultpb.needSupressSuccess {
					defaultpb.PrintSupressedSuccess(completed)
				} else {
					pterm.Success.WithWriter(writerFabric.GetWriter()).Println(completed)
				}

				increment := int(math.Round(msg.Progress*100) - float64(p.Current))

				if increment == 0 {
					increment = inc
				}

				if p.Current+increment > 100 {
					increment = 100 - p.Current
				}

				p.Add(increment)

				if !p.IsActive {
					// need to finilize pb
					if defaultpb.needSupressSuccess {
						if err := defaultpb.LogBox.Stop(); err != nil {
							return
						}
						defaultpb.FinalizeSuccess()
						return
					}
				}

				lastCompleted = completed
				p.UpdateTitle(phaseToString(msg, false))

				if defaultpb.CheckPhase(string(msg.CurrentPhase), string(msg.CurrentSubPhase)) {
					estimatedHeight := defaultpb.availableHeight
					if estimatedHeight > 10 {
						estimatedHeight = 10
					}
					if estimatedHeight <= 0 {
						continue
					} else {
						logBox := newLogBox(writerFabric, defaultpb.LogBox.logChan, estimatedHeight).WithStatusString(defaultpb.LogBox.getStatusString())
						if err := logBox.Start(); err != nil {
							return
						}
						defaultpb.LogBox = logBox
						go logBox.Update()
					}
				}
			}
		}
	}
}

// if Progressbar used, this func allows to print to new MultiPrinter Writer
func InfoF(format string, a ...any) {
	var writer io.Writer
	if defaultpb.LogBox != nil {
		writer = defaultpb.LogBox.ShiftDown()
	} else {
		writer = defaultpb.WriterFabric.GetWriter()
	}

	pterm.Info.WithWriter(writer).Printf(format, a...)
	if defaultpb.needSupressSuccess {
		defaultpb.successedPhases = append(defaultpb.successedPhases, holdedMessage{mType: infoMsgType, message: fmt.Sprintf(format, a...)})
	}
}

func WarnF(format string, a ...any) {
	var writer io.Writer
	if defaultpb.LogBox != nil {
		writer = defaultpb.LogBox.ShiftDown()
	} else {
		writer = defaultpb.WriterFabric.GetWriter()
	}
	pterm.Warning.WithWriter(writer).Printf(format, a...)
}

func ErrorF(format string, a ...any) {
	if defaultpb != nil {
		var writer io.Writer
		if defaultpb.LogBox != nil {
			writer = defaultpb.LogBox.ShiftDown()
		} else {
			writer = defaultpb.WriterFabric.GetWriter()
		}
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
	subphasesMap[phases.InstallKubernetesSubPhaseModulesPreparation] = "Prepare modules for bashible bundle"
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
	time.Sleep(300 * time.Millisecond)

	if pb.LogBox != nil {
		if err := pb.LogBox.Stop(); err != nil {
			log.WarnF("failed to stop log box: %s\n", err.Error())
		}
	}

	if pb.ProgressBarPrinter.IsActive {
		pb.ProgressBarPrinter.Add(100 - pb.ProgressBarPrinter.Current)
		if _, err := pb.MultiPrinter.Stop(); err != nil {
			log.WarnF("failed to stop multi printer: %v", err)
		}
	}
}
