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
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// Implementation of rolling logs from log.Info*
// To keep them from being removed from screen during pb/spinners refresh, it will be just a 10 spinners

type LogBox struct {
	writerFabric        *WriterFabric
	SpinnerPrinterArray [11]*pterm.SpinnerPrinter
	writerArray         [11]io.Writer
	logChan             chan string
	stopChan            chan struct{}
	mu                  sync.Mutex
	lastFilledString    int
	started             bool
	status              string
}

func newLogBox(writerFabric *WriterFabric, logChan chan string) *LogBox {
	spinnerArray := [11]*pterm.SpinnerPrinter{}
	writerArray := [11]io.Writer{}
	spinnerStyle := pterm.NewStyle(pterm.FgDarkGray)
	for i := range 11 {
		style := spinnerStyle
		if i == 0 {
			style = pterm.NewStyle(pterm.FgLightYellow)
		}
		writer := writerFabric.GetWriter()
		staticSpinner := pterm.DefaultSpinner.
			WithSequence(" ").
			WithDelay(time.Hour).
			WithWriter(writer).
			WithMessageStyle(style).
			WithRemoveWhenDone(true).
			WithShowTimer(false)
		spinnerArray[i] = staticSpinner
		writerArray[i] = writer
	}

	stopCh := make(chan struct{})

	return &LogBox{
		writerFabric:        writerFabric,
		SpinnerPrinterArray: spinnerArray,
		logChan:             logChan,
		stopChan:            stopCh,
		writerArray:         writerArray,
		lastFilledString:    0,
	}
}

func (b *LogBox) Start() error {
	for i := range 11 {
		msg := ""
		if i == 0 {
			msg = b.status
		}
		_, err := b.SpinnerPrinterArray[i].Start(msg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *LogBox) Stop() error {
	if b.started {
		b.stopChan <- struct{}{}
		for i := range 11 {
			err := b.SpinnerPrinterArray[i].Stop()
			if err != nil {
				return err
			}
			b.writerFabric.PutWriter(b.writerArray[i])
		}
	}

	return nil
}

func (b *LogBox) putMsg(msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.lastFilledString != 10 {
		b.SpinnerPrinterArray[b.lastFilledString+1].UpdateText(msg)
		b.lastFilledString++
	} else {
		for i := range 9 {
			b.SpinnerPrinterArray[i+1].UpdateText(b.SpinnerPrinterArray[i+2].Text)
		}
		b.SpinnerPrinterArray[10].UpdateText(msg)
	}
}

func (b *LogBox) Update() {
	b.started = true
	for {
		select {
		case <-b.stopChan:
			b.started = false
			return
		case msg, ok := <-b.logChan:
			if !ok {
				continue
			}

			new := strings.TrimSuffix(msg, "\n")
			splitted := strings.Split(new, "\n")
			for _, s := range splitted {
				s = "    " + s
				b.putMsg(s)
			}
		}
	}
}

// set the text of first string to given one
func (b *LogBox) WithStatusString(s string) *LogBox {
	if b == nil {
		return nil
	}

	b.status = s

	if len(b.SpinnerPrinterArray) == 0 {
		return b
	}

	b.SpinnerPrinterArray[0].UpdateText(s)

	return b
}

func (b *LogBox) getStatusString() string {
	if b == nil {
		return ""
	}

	return b.status
}

func (b *LogBox) ShiftDown() io.Writer {
	b.mu.Lock()
	defer b.mu.Unlock()

	wr := b.writerArray[0]
	wrArr := append(b.writerArray[1:], b.writerFabric.GetWriter())

	for i := range 11 {
		b.SpinnerPrinterArray[i].SetWriter(wrArr[i])
		b.writerArray[i] = wrArr[i]
	}

	return wr
}

func (b *LogBox) ShiftUp(w io.Writer) {
	b.mu.Lock()
	defer b.mu.Unlock()

	wr := b.writerArray[0]
	copy(b.writerArray[1:], b.writerArray[0:10])
	b.writerArray[0] = w

	for i := range 11 {
		b.SpinnerPrinterArray[i].SetWriter(b.writerArray[i])
	}

	b.writerFabric.PutWriter(wr)
}

type WriterFabric struct {
	mp     *pterm.MultiPrinter
	witers []io.Writer
}

func newWriterFabric(mp *pterm.MultiPrinter) WriterFabric {
	return WriterFabric{
		mp:     mp,
		witers: make([]io.Writer, 0),
	}
}

func (w *WriterFabric) GetWriter() io.Writer {
	if len(w.witers) != 0 {
		writer := w.witers[0]
		w.witers = w.witers[1:]

		return writer
	}

	return w.mp.NewWriter()
}

func (w *WriterFabric) PutWriter(writer io.Writer) {
	w.witers = append(w.witers, writer)
}

func (w *WriterFabric) Cleanup() {
	if w == nil {
		return
	}

	if len(w.witers) > 0 {
		for range w.witers {
			fmt.Print("\033[1F\033[2K")
		}
	}
}
