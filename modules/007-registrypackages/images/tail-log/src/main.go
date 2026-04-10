/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	initialTailBytes = 64 * 1024
	initialTailLines = 100
	pollInterval     = 200 * time.Millisecond
)

func tailLines(data []byte, count int) []byte {
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}

	return bytes.Join(lines, nil)
}

func reopenLogFile(file *os.File, path string) (*os.File, error) {
	if file != nil {
		_ = file.Close()
	}

	return os.Open(path)
}

func writeInitialTail(f *os.File, w io.Writer) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}

	start := info.Size() - initialTailBytes
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if start > 0 {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}
	}

	_, err = w.Write(tailLines(data, initialTailLines))
	return err
}

func streamLog(ctx context.Context, w io.Writer, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() {
		if f != nil {
			_ = f.Close()
		}
	}()

	if err := writeInitialTail(f, w); err != nil {
		return
	}

	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	position, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}

	buf := make([]byte, 4096)
	for ctx.Err() == nil {
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			position += int64(n)
			if flusher != nil {
				flusher.Flush()
			}
			continue
		}

		if err != nil && err != io.EOF {
			time.Sleep(2 * pollInterval)
		} else {
			time.Sleep(pollInterval)
		}

		currentInfo, currentErr := f.Stat()
		latestInfo, latestErr := os.Stat(path)
		if currentErr != nil || latestErr != nil {
			continue
		}

		if !os.SameFile(currentInfo, latestInfo) {
			newFile, openErr := reopenLogFile(f, path)
			if openErr != nil {
				continue
			}
			f = newFile
			position = 0
			continue
		}

		if latestInfo.Size() < position {
			if _, seekErr := f.Seek(0, io.SeekStart); seekErr == nil {
				position = 0
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: logtail <file>")
		os.Exit(1)
	}

	logFile := os.Args[1]
	addr := "0.0.0.0:8000"
	fmt.Printf("Streaming %s on http://%s\n", logFile, addr)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		streamLog(r.Context(), w, logFile)
	})

	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
