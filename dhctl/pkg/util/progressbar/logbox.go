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

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// Implementation of rolling logs from log.Info*
// To keep them from being removed from screen during pb/spinners refresh, it will be just a 2 spinners

type LogBox struct {
	writerFabric        *WriterFabric
	SpinnerPrinterArray [2]*pterm.SpinnerPrinter
	writerArray         [2]io.Writer
	text                []string
	logChan             chan string
	stopChan            chan struct{}
	mu                  sync.Mutex
	lastFilledString    int
	started             bool
	status              string
	linesNumber         int
}

func newLogBox(writerFabric *WriterFabric, logChan chan string, linesNumber int) *LogBox {
	spinnerArray := [2]*pterm.SpinnerPrinter{}
	writerArray := [2]io.Writer{}
	spinnerStyle := pterm.NewStyle(pterm.FgDarkGray)
	for i := range 2 {
		style := spinnerStyle
		if i == 0 {
			style = pterm.NewStyle(pterm.FgLightYellow)
		}
		writer := writerFabric.GetWriter()
		staticSpinner := pterm.DefaultSpinner.
			WithSequence("").
			WithDelay(time.Hour).
			WithWriter(writer).
			WithMessageStyle(style).
			WithRemoveWhenDone(true).
			WithShowTimer(false)
		spinnerArray[i] = staticSpinner
		writerArray[i] = writer
	}

	stopCh := make(chan struct{})
	text := make([]string, linesNumber)

	return &LogBox{
		writerFabric:        writerFabric,
		SpinnerPrinterArray: spinnerArray,
		logChan:             logChan,
		stopChan:            stopCh,
		writerArray:         writerArray,
		lastFilledString:    0,
		text:                text,
		linesNumber:         linesNumber,
	}
}

func (b *LogBox) Start() error {
	for i := range 2 {
		msg := ""
		if i == 0 {
			msg = b.status
		}
		_, err := b.SpinnerPrinterArray[i].Start(msg)
		if err != nil {
			return err
		}
	}

	log.WithLogSending(true)

	// if defaultpb is not initialized yet, availableHeight must be set by yourself
	if defaultpb != nil {
		// LogBox should consume 11 lines total. 2 by calling writerFabric.GetWriter() and 9 additional lines here
		defaultpb.availableHeight = defaultpb.availableHeight + 1 - b.linesNumber
	}

	return nil
}

func (b *LogBox) Stop() error {
	if b == nil {
		return nil
	}

	if b.started {
		log.WithLogSending(false)
		b.stopChan <- struct{}{}
		// waiting for LogBox will be stopped
		for b.started {
			time.Sleep(50 * time.Millisecond)
		}

		for i := range 2 {
			err := b.SpinnerPrinterArray[i].Stop()
			if err != nil {
				return err
			}
			b.writerFabric.PutWriter(b.writerArray[i])
		}
	}

	b.text = make([]string, b.linesNumber)
	defaultpb.availableHeight += (b.linesNumber - 1)

	return nil
}

func (b *LogBox) Cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.text = make([]string, b.linesNumber)
	b.SpinnerPrinterArray[1].UpdateText("")
}

func (b *LogBox) putMsg(msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.lastFilledString != b.linesNumber-1 {
		b.text[b.lastFilledString+1] = msg
		b.lastFilledString++
	} else {
		for i := range b.linesNumber - 1 {
			b.text[i] = b.text[i+1]
		}
		b.text[b.linesNumber-1] = msg
	}
	resText := strings.Join(b.text, "\n")
	b.SpinnerPrinterArray[1].UpdateText(resText)
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
				b.started = false
				return
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

	b.SpinnerPrinterArray[1].UpdateText("")
	b.lastFilledString = 0

	wr := b.writerArray[0]
	wrArr := append(b.writerArray[1:], b.writerFabric.GetWriter())

	for i := range 2 {
		b.SpinnerPrinterArray[i].SetWriter(wrArr[i])
		b.writerArray[i] = wrArr[i]
	}

	return wr
}

func (b *LogBox) ShiftUp(w io.Writer) {
	b.mu.Lock()
	defer b.mu.Unlock()

	wr := b.writerArray[0]
	copy(b.writerArray[1:], b.writerArray[0:2])
	b.writerArray[0] = w

	for i := range 2 {
		b.SpinnerPrinterArray[i].SetWriter(b.writerArray[i])
	}

	b.writerFabric.PutWriter(wr)
}

type WriterFabric struct {
	mp         *pterm.MultiPrinter
	writers    []io.Writer
	allWriters []io.Writer
	mu         sync.Mutex
}

func newWriterFabric(mp *pterm.MultiPrinter) WriterFabric {
	return WriterFabric{
		mp:         mp,
		writers:    make([]io.Writer, 0),
		allWriters: make([]io.Writer, 0),
	}
}

func (w *WriterFabric) GetWriter() io.Writer {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.writers) != 0 {
		writer := w.writers[0]
		w.writers = w.writers[1:]

		return writer
	}

	// if defaultpb is not initialized yet, availableHeight must be set by yourself
	if defaultpb != nil {
		defaultpb.availableHeight--
	}

	wr := w.mp.NewWriter()
	w.allWriters = append(w.allWriters, wr)
	return wr
}

func (w *WriterFabric) PutWriter(writer io.Writer) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writers = append(w.writers, writer)
}

func (w *WriterFabric) Cleanup() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.writers) > 0 {
		for range w.writers {
			fmt.Print("\033[1F\033[2K")
		}
	}
}
