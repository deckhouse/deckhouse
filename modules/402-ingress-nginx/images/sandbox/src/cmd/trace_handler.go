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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/criyle/go-sandbox/pkg/seccomp/libseccomp"
	"github.com/criyle/go-sandbox/ptracer"
	sbptrace "github.com/criyle/go-sandbox/runner/ptrace"
)

type sandboxTraceHandler struct {
	ShowDetails bool
	Unsafe      bool
	Base        sbptrace.Handler
}

func newSandboxTraceHandler(base sbptrace.Handler, debug bool) *sandboxTraceHandler {
	return &sandboxTraceHandler{
		ShowDetails: debug,
		Base:        base,
	}
}

func (h *sandboxTraceHandler) Debug(v ...interface{}) {
	if h.ShowDetails {
		log.Print(v...)
	}
}

func (h *sandboxTraceHandler) getString(ctx *ptracer.Context, addr uint) string {
	return absPath(ctx.Pid, ctx.GetString(uintptr(addr)))
}

func (h *sandboxTraceHandler) checkOpen(ctx *ptracer.Context, addr uint, flags uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	isReadOnly := (flags&syscall.O_ACCMODE == syscall.O_RDONLY) &&
		(flags&syscall.O_CREAT == 0) &&
		(flags&syscall.O_EXCL == 0) &&
		(flags&syscall.O_TRUNC == 0)

	h.Debug("open: ", fn, getFileMode(flags))
	if isReadOnly {
		return h.Base.CheckRead(fn)
	}
	return h.Base.CheckWrite(fn)
}

func (h *sandboxTraceHandler) checkRead(ctx *ptracer.Context, addr uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	h.Debug("check read: ", fn)
	return h.Base.CheckRead(fn)
}

func (h *sandboxTraceHandler) checkWrite(ctx *ptracer.Context, addr uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	h.Debug("check write: ", fn)
	return h.Base.CheckWrite(fn)
}

func (h *sandboxTraceHandler) checkStat(ctx *ptracer.Context, addr uint) ptracer.TraceAction {
	fn := h.getString(ctx, addr)
	h.Debug("check stat: ", fn)
	return h.Base.CheckStat(fn)
}

func (h *sandboxTraceHandler) Handle(ctx *ptracer.Context) ptracer.TraceAction {
	syscallNo := ctx.SyscallNo()
	syscallName, err := libseccomp.ToSyscallName(syscallNo)
	h.Debug("syscall: ", syscallNo, " ", syscallName, " ", err)
	if err != nil {
		h.Debug("invalid syscall no")
		return ptracer.TraceKill
	}

	var action ptracer.TraceAction
	switch syscallName {
	case "open":
		action = h.checkOpen(ctx, ctx.Arg0(), ctx.Arg1())
	case "openat", "openat2":
		action = h.checkOpen(ctx, ctx.Arg1(), ctx.Arg2())

	case "readlink":
		action = h.checkRead(ctx, ctx.Arg0())
	case "readlinkat":
		action = h.checkRead(ctx, ctx.Arg1())

	case "unlink":
		action = h.checkWrite(ctx, ctx.Arg0())
	case "unlinkat":
		action = h.checkWrite(ctx, ctx.Arg1())

	case "access":
		action = h.checkStat(ctx, ctx.Arg0())
	case "faccessat", "faccessat2":
		action = h.checkStat(ctx, ctx.Arg1())

	case "stat", "stat64":
		action = h.checkStat(ctx, ctx.Arg0())
	case "lstat", "lstat64":
		action = h.checkStat(ctx, ctx.Arg0())
	case "statx", "fstatat", "fstatat64", "newfstatat":
		action = h.checkStat(ctx, ctx.Arg1())

	case "execve":
		action = h.checkRead(ctx, ctx.Arg0())
	case "execveat":
		action = h.checkRead(ctx, ctx.Arg1())

	case "chmod":
		action = h.checkWrite(ctx, ctx.Arg0())
	case "rename":
		action = h.checkWrite(ctx, ctx.Arg0())

	default:
		action = h.Base.CheckSyscall(syscallName)
		if h.Unsafe && action == ptracer.TraceKill {
			action = ptracer.TraceBan
		}
		if !h.ShowDetails && action == ptracer.TraceBan {
			log.Printf("deny syscall=%q", syscallName)
			action = ptracer.TraceKill
		}
	}

	switch action {
	case ptracer.TraceAllow:
		h.Debug("decision: allow syscall=", syscallName)
		return ptracer.TraceAllow
	case ptracer.TraceBan:
		h.Debug("decision: soft-ban syscall=", syscallName)
		return softBanSyscall(ctx)
	default:
		h.Debug("decision: kill syscall=", syscallName)
		return ptracer.TraceKill
	}
}

func getFileMode(flags uint) string {
	switch flags & syscall.O_ACCMODE {
	case syscall.O_RDONLY:
		return "r "
	case syscall.O_WRONLY:
		return "w "
	case syscall.O_RDWR:
		return "wr"
	default:
		return "??"
	}
}

func softBanSyscall(ctx *ptracer.Context) ptracer.TraceAction {
	ctx.SetReturnValue(-int(sbptrace.BanRet))
	return ptracer.TraceBan
}

func getProcCwd(pid int) string {
	fileName := "/proc/self/cwd"
	if pid > 0 {
		fileName = fmt.Sprintf("/proc/%d/cwd", pid)
	}
	s, err := os.Readlink(fileName)
	if err != nil {
		return ""
	}
	return s
}

func absPath(pid int, p string) string {
	if !filepath.IsAbs(p) {
		return filepath.Join(getProcCwd(pid), p)
	}
	return filepath.Clean(p)
}
