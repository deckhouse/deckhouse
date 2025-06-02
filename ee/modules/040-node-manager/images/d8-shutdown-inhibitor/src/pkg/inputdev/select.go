//go:build linux

/*
Copyright 2025 Flant JSC

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

package inputdev

import (
	"syscall"
)

// FD_SET sets the bit for the file descriptor fd in the bitmask FdSet.
func FD_SET(fd int, fdSet *syscall.FdSet) {
	fdIdx := fd / 64
	fdBitMask := int64(1 << (fd % 64))
	fdSet.Bits[fdIdx] |= fdBitMask
	// syscall.FD_SET(fd, p)
}

// FD_ISSET examines if bit is set for the file descriptor fd in the bitmask FdSet.
func FD_ISSET(fd int, fdSet *syscall.FdSet) bool {
	fdIdx := fd / 64
	fdBitMask := int64(1 << (fd % 64))
	return (fdSet.Bits[fdIdx] & fdBitMask) != 0
}

// FD_ZERO cleans the bitmask FdSet to initialize bitmask after modification from the Select call.
func FD_ZERO(fdSet *syscall.FdSet) {
	for i := range fdSet.Bits {
		fdSet.Bits[i] = 0
	}
}

// InitFdSet cleans and initializes FdSet bitmask with the provided file descriptors.
func InitFdSet(fdSet *syscall.FdSet, fds ...int) {
	FD_ZERO(fdSet)
	for _, fd := range fds {
		FD_SET(fd, fdSet)
	}
}
