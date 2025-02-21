//go:build linux

/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
