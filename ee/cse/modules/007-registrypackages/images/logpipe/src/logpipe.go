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
	"bufio"
	"flag"
	"fmt"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	file := flag.String("file", "", "log file output path")
	maxSize := flag.Int("max-size", 10, "maximum log file size in megabytes")
	maxBackups := flag.Int("max-backups", 5, "maximum number of old log files to retain")
	maxAge := flag.Int("max-age", 30, "maximum number of days to retain old log files")
	compress := flag.Bool("compress", false, "compress old log files")

	flag.Usage = func() {
		fmt.Printf("Usage: logpipe [options]\n\n")
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *file == "" {
		fmt.Println("Error: -file option is required")
		flag.Usage()
		return
	}

	lumberjackLogger := lumberjack.Logger{
		Filename:   *file,
		MaxSize:    *maxSize,
		MaxBackups: *maxBackups,
		MaxAge:     *maxAge,
		Compress:   *compress,
	}
	defer lumberjackLogger.Close()
	scanner := bufio.NewScanner(bufio.NewReaderSize(os.Stdin, 1024*1024))
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := lumberjackLogger.Write([]byte(line + "\n")); err != nil {
			fmt.Fprintln(os.Stderr, "Error writing to log file:", err)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading from stdin:", err)
		os.Exit(1)
	}
}
