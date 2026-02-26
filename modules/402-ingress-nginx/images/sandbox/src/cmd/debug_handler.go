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
	"log"

	"github.com/criyle/go-sandbox/ptracer"
	"github.com/criyle/go-sandbox/runner/ptrace"
)

func withDebugHandler(base ptrace.Handler, crashOnDeny bool) ptrace.Handler {
	return &debugHandler{base: base, crashOnDeny: crashOnDeny}
}

type debugHandler struct {
	base        ptrace.Handler
	crashOnDeny bool
}

func (h *debugHandler) CheckRead(path string) ptracer.TraceAction {
	action := h.base.CheckRead(path)
	log.Printf("[sandbox handler debug] read path=%q action=%d", path, action)
	if h.crashOnDeny && action != ptracer.TraceAllow {
		log.Printf("[sandbox handler debug] crash-on-deny enabled: forcing kill for read path=%q", path)
		return ptracer.TraceKill
	}
	return action
}

func (h *debugHandler) CheckWrite(path string) ptracer.TraceAction {
	action := h.base.CheckWrite(path)
	log.Printf("[sandbox handler debug] write path=%q action=%d", path, action)
	if h.crashOnDeny && action != ptracer.TraceAllow {
		log.Printf("[sandbox handler debug] crash-on-deny enabled: forcing kill for write path=%q", path)
		return ptracer.TraceKill
	}
	return action
}

func (h *debugHandler) CheckStat(path string) ptracer.TraceAction {
	action := h.base.CheckStat(path)
	log.Printf("[sandbox handler debug] stat path=%q action=%d", path, action)
	if h.crashOnDeny && action != ptracer.TraceAllow {
		log.Printf("[sandbox handler debug] crash-on-deny enabled: forcing kill for stat path=%q", path)
		return ptracer.TraceKill
	}
	return action
}

func (h *debugHandler) CheckSyscall(syscallName string) ptracer.TraceAction {
	action := h.base.CheckSyscall(syscallName)
	log.Printf("[sandbox handler debug] syscall=%q action=%d", syscallName, action)
	if h.crashOnDeny && action != ptracer.TraceAllow {
		log.Printf("[sandbox handler debug] crash-on-deny enabled: forcing kill for syscall=%q", syscallName)
		return ptracer.TraceKill
	}
	return action
}
