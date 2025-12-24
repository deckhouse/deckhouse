// Copyright 2025 Flant JSC
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

package main

import (
	"fmt"
	"os"
	"runtime/pprof"
	"runtime/trace"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func enableTrace() (func(), error) {
	traceFileName := os.Getenv("DHCTL_TRACE")
	cpuProfileFileName := traceFileName + ".prof.cpu"

	if traceFileName == "" || traceFileName == "0" || traceFileName == "no" {
		return func() {}, nil
	}
	if traceFileName == "1" || traceFileName == "yes" {
		traceFileName = "trace.out"
		cpuProfileFileName = "pprof.cpu"
	}

	fns := make([]func(), 0)

	traceF, err := os.Create(traceFileName)
	if err != nil {
		return func() {}, fmt.Errorf("Failed to create trace output file '%s': %v", traceFileName, err)
	}

	fns = append([]func(){
		func() {
			if err := traceF.Close(); err != nil {
				log.InfoF("failed to close trace file '%s': %v", traceFileName, err)
				os.Exit(1)
			}
		},
	}, fns...)

	profCPU, err := os.Create(cpuProfileFileName)
	if err != nil {
		return func() {}, fmt.Errorf("Failed to create pprof cpu file '%s': %v", cpuProfileFileName, err)
	}

	fns = append([]func(){
		func() {
			if err := profCPU.Close(); err != nil {
				log.InfoF("failed to close pprof cpu file '%s': %v", cpuProfileFileName, err)
				os.Exit(1)
			}
		},
	}, fns...)

	if err := trace.Start(traceF); err != nil {
		return func() {}, fmt.Errorf("failed to start trace to '%s': %v", traceFileName, err)
	}
	fns = append([]func(){
		trace.Stop,
	}, fns...)

	if err := pprof.StartCPUProfile(profCPU); err != nil {
		return func() {}, fmt.Errorf("Failed to start profile cpu to '%s': %v", cpuProfileFileName, err)
	}

	fns = append([]func(){
		pprof.StopCPUProfile,
	}, fns...)

	return func() {
		for _, fn := range fns {
			fn()
		}
	}, nil
}
