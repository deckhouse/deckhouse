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
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/criyle/go-sandbox/pkg/seccomp/libseccomp"
	"github.com/criyle/go-sandbox/ptracer"
	sbptrace "github.com/criyle/go-sandbox/runner/ptrace"
	"golang.org/x/sys/unix"
)

const sandboxAllowDNSToArg = "--allow-dns-to"

var (
	sandboxAllowedLoopbackConnects = map[netip.AddrPort]struct{}{
		netip.MustParseAddrPort("127.0.0.1:65535"): {},
		netip.MustParseAddrPort("[::1]:65535"):     {},
	}
)

const sandboxAllowedProbePort = 65535

type sandboxDNSPolicy struct {
	server netip.AddrPort
}

type sandboxTraceHandler struct {
	ShowDetails bool
	Unsafe      bool
	Base        sbptrace.Handler
	DNSPolicy   *sandboxDNSPolicy

	allowedDNSFDs map[uint]struct{}
}

func parseSandboxDNSPolicy(value string) (*sandboxDNSPolicy, error) {
	server, err := netip.ParseAddrPort(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid addr:port: %w", sandboxAllowDNSToArg, err)
	}
	if !server.IsValid() {
		return nil, fmt.Errorf("%s must not be empty", sandboxAllowDNSToArg)
	}
	if server.Port() == 0 {
		return nil, fmt.Errorf("%s port must be non-zero", sandboxAllowDNSToArg)
	}

	return &sandboxDNSPolicy{server: server}, nil
}

func parseSandboxArgs(argv []string) (*sandboxDNSPolicy, []string, error) {
	var dnsPolicy *sandboxDNSPolicy

	i := 0
	for i < len(argv) {
		arg := argv[i]
		switch {
		case arg == "--":
			return dnsPolicy, argv[i+1:], nil
		case !strings.HasPrefix(arg, "--"):
			return dnsPolicy, argv[i:], nil
		case arg == sandboxAllowDNSToArg:
			if i+1 >= len(argv) {
				return nil, nil, fmt.Errorf("%s requires addr:port value", sandboxAllowDNSToArg)
			}
			policy, err := parseSandboxDNSPolicy(argv[i+1])
			if err != nil {
				return nil, nil, err
			}
			dnsPolicy = policy
			i += 2
		case strings.HasPrefix(arg, sandboxAllowDNSToArg+"="):
			policy, err := parseSandboxDNSPolicy(strings.TrimPrefix(arg, sandboxAllowDNSToArg+"="))
			if err != nil {
				return nil, nil, err
			}
			dnsPolicy = policy
			i++
		default:
			return nil, nil, fmt.Errorf("unknown sandbox argument: %s", arg)
		}
	}

	return dnsPolicy, argv[i:], nil
}

func sandboxExtraTraceSyscalls(dnsPolicy *sandboxDNSPolicy) []string {
	if dnsPolicy == nil {
		return nil
	}
	return []string{"connect", "sendmsg", "sendto", "recvfrom", "recvmsg", "close"}
}

func newSandboxTraceHandler(base sbptrace.Handler, dnsPolicy *sandboxDNSPolicy, debug bool) *sandboxTraceHandler {
	return &sandboxTraceHandler{
		ShowDetails:   debug,
		Base:          base,
		DNSPolicy:     dnsPolicy,
		allowedDNSFDs: make(map[uint]struct{}),
	}
}

func (p *sandboxDNSPolicy) allowsConnectDestination(dst netip.AddrPort) bool {
	if p == nil {
		return false
	}
	if dst == p.server {
		return true
	}
	_, ok := sandboxAllowedLoopbackConnects[dst]
	if ok {
		return true
	}
	return dst.Port() == sandboxAllowedProbePort
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

func (h *sandboxTraceHandler) checkSendTo(ctx *ptracer.Context) ptracer.TraceAction {
	if h.DNSPolicy == nil {
		return ptracer.TraceKill
	}

	addrPtr := ctx.Arg4()
	addrLen := ctx.Arg5()
	if addrPtr == 0 {
		h.Debug("deny sendto with nil destination address")
		return ptracer.TraceKill
	}

	dst, err := readSockaddrAddrPort(ctx.Pid, uintptr(addrPtr), addrLen)
	if err != nil {
		h.Debug("deny sendto: failed to read destination sockaddr: ", err)
		return ptracer.TraceKill
	}
	if dst != h.DNSPolicy.server {
		h.Debug("deny sendto: destination is not allowed: ", dst)
		return ptracer.TraceKill
	}

	fd := ctx.Arg0()
	h.allowedDNSFDs[fd] = struct{}{}
	h.Debug("allow sendto: fd=", fd, " dst=", dst)
	return ptracer.TraceAllow
}

func (h *sandboxTraceHandler) checkConnect(ctx *ptracer.Context) ptracer.TraceAction {
	if h.DNSPolicy == nil {
		return ptracer.TraceKill
	}

	fd := ctx.Arg0()
	addrPtr := ctx.Arg1()
	addrLen := ctx.Arg2()
	if addrPtr == 0 {
		h.Debug("deny connect with nil destination address")
		return ptracer.TraceKill
	}

	dst, err := readSockaddrAddrPort(ctx.Pid, uintptr(addrPtr), addrLen)
	if err != nil {
		h.Debug("deny connect: failed to read destination sockaddr: ", err)
		return ptracer.TraceKill
	}
	if !h.DNSPolicy.allowsConnectDestination(dst) {
		h.Debug("deny connect: destination is not allowed: ", dst)
		return ptracer.TraceKill
	}

	if dst == h.DNSPolicy.server {
		h.allowedDNSFDs[fd] = struct{}{}
	}
	h.Debug("allow connect: fd=", fd, " dst=", dst)
	return ptracer.TraceAllow
}

func (h *sandboxTraceHandler) checkSendMsg(ctx *ptracer.Context) ptracer.TraceAction {
	if h.DNSPolicy == nil {
		return ptracer.TraceKill
	}

	fd := ctx.Arg0()
	msg, err := readMsghdr(ctx.Pid, uintptr(ctx.Arg1()))
	if err != nil {
		h.Debug("deny sendmsg: failed to read msghdr: ", err)
		return ptracer.TraceKill
	}

	if msg.Name != nil && msg.Namelen > 0 {
		dst, err := readSockaddrAddrPort(ctx.Pid, uintptr(unsafe.Pointer(msg.Name)), uint(msg.Namelen))
		if err != nil {
			h.Debug("deny sendmsg: failed to read destination sockaddr: ", err)
			return ptracer.TraceKill
		}
		if dst != h.DNSPolicy.server {
			h.Debug("deny sendmsg: destination is not allowed: ", dst)
			return ptracer.TraceKill
		}
		h.allowedDNSFDs[fd] = struct{}{}
		h.Debug("allow sendmsg with destination: fd=", fd, " dst=", dst)
		return ptracer.TraceAllow
	}

	if _, ok := h.allowedDNSFDs[fd]; !ok {
		h.Debug("deny sendmsg on fd without prior allowed DNS connect/sendto: ", fd)
		return ptracer.TraceKill
	}
	h.Debug("allow sendmsg on previously approved DNS fd: ", fd)
	return ptracer.TraceAllow
}

func (h *sandboxTraceHandler) checkDNSReply(fd uint) ptracer.TraceAction {
	if h.DNSPolicy == nil {
		return ptracer.TraceKill
	}
	if _, ok := h.allowedDNSFDs[fd]; !ok {
		h.Debug("deny receive on fd without prior allowed DNS sendto: ", fd)
		return ptracer.TraceKill
	}
	return ptracer.TraceAllow
}

func (h *sandboxTraceHandler) checkClose(fd uint) ptracer.TraceAction {
	delete(h.allowedDNSFDs, fd)
	return ptracer.TraceAllow
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

	case "connect":
		action = h.checkConnect(ctx)
	case "sendmsg":
		action = h.checkSendMsg(ctx)
	case "sendto":
		action = h.checkSendTo(ctx)
	case "recvfrom", "recvmsg":
		action = h.checkDNSReply(ctx.Arg0())
	case "close":
		action = h.checkClose(ctx.Arg0())

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

func readSockaddrAddrPort(pid int, addr uintptr, addrLen uint) (netip.AddrPort, error) {
	readLen := int(addrLen)
	if readLen <= 0 {
		return netip.AddrPort{}, errors.New("empty sockaddr")
	}
	if readLen > 28 {
		readLen = 28
	}

	buf := make([]byte, readLen)
	if _, err := syscall.PtracePeekData(pid, addr, buf); err != nil {
		return netip.AddrPort{}, err
	}

	return parseSockaddrAddrPort(buf)
}

func readMsghdr(pid int, addr uintptr) (unix.Msghdr, error) {
	var msg unix.Msghdr
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&msg)), unsafe.Sizeof(msg))
	if _, err := syscall.PtracePeekData(pid, addr, buf); err != nil {
		return unix.Msghdr{}, err
	}
	return msg, nil
}

func parseSockaddrAddrPort(buf []byte) (netip.AddrPort, error) {
	if len(buf) < 4 {
		return netip.AddrPort{}, errors.New("sockaddr is too short")
	}

	family := binary.LittleEndian.Uint16(buf[:2])
	switch family {
	case syscall.AF_INET:
		if len(buf) < 16 {
			return netip.AddrPort{}, errors.New("short sockaddr_in")
		}
		port := binary.BigEndian.Uint16(buf[2:4])
		addr := netip.AddrFrom4([4]byte{buf[4], buf[5], buf[6], buf[7]})
		return netip.AddrPortFrom(addr, port), nil
	case syscall.AF_INET6:
		if len(buf) < 28 {
			return netip.AddrPort{}, errors.New("short sockaddr_in6")
		}
		port := binary.BigEndian.Uint16(buf[2:4])
		var ip [16]byte
		copy(ip[:], buf[8:24])
		addr := netip.AddrFrom16(ip)
		return netip.AddrPortFrom(addr, port), nil
	default:
		return netip.AddrPort{}, fmt.Errorf("unsupported sockaddr family: %d", family)
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
