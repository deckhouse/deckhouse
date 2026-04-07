/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

	scanner := bufio.NewScanner(bufio.NewReaderSize(os.Stdin, 1024*1024))
	for scanner.Scan() {
		line := scanner.Text()
		if _, err := lumberjackLogger.Write([]byte(line + "\n")); err != nil {
			fmt.Fprintln(os.Stderr, "Error writing to log file:", err)
			lumberjackLogger.Close()
			os.Exit(1)
		}
	}
	lumberjackLogger.Close()
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading from stdin:", err)
		os.Exit(1)
	}
}
