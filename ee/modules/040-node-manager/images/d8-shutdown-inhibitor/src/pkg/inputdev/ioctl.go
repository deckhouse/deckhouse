/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inputdev

// Directions
const (
	_IOC_NONE  = 0
	_IOC_WRITE = 1
	_IOC_READ  = 2
)

// _IOC is a copy of _IOC macro from linux/ioctl.h
// It encodes command into uint to use as a second argument in ioctl syscall.
//
// Encoding: 32 bits total, command in lower 16 bits,
// Command is a combination of two bytes that driver recognizes as a command number.
// Size of the parameter structure in the lower 14 bits of the upper 16 bits.
// Direction in the upper 2 bits of the upper 16 bits.
//
// Encoded bits result:
// DDSSSSSS SSSSSSSS CCCCCCCC CCCCCCCC
//
// Example:
// Command to read input device name.
// - Direction is read == 2
// - ioctl parameter is a pointer to buffer, we need to encode size of this buffer, e.g. 80 bytes.
// - Command should have value of symbol 'E' as the first byte.
// - Second byte to read device name is 0x06.
// DD == 0b10 (2)
// SSSSSS SSSSSSSS = 80 = 0x50 == 000000 01010000
// 'E' = 0x45 = 0b01000101
// type 0x06 == 0b00000110
// DDSSSSSS SSSSSSSS CCCCCCCC CCCCCCCC
// 10000000 01010000 01000101 00000110 == 0x80504506
//
// Original:
//
//	#define _IOC(dir,type,nr,size) \
//	     (((dir)  << _IOC_DIRSHIFT) | \    // 30
//	      ((type) << _IOC_TYPESHIFT) | \   //  8
//	      ((nr)   << _IOC_NRSHIFT) | \     //  0
//	      ((size) << _IOC_SIZESHIFT))      // 16
func _IOC(dir, typ, nr uint, size int) uint {
	return ((dir) << _IOC_DIRSHIFT) |
		((typ) << _IOC_TYPESHIFT) |
		((nr) << _IOC_NRSHIFT) |
		((uint(size)) << _IOC_SIZESHIFT)
}

const (
	_IOC_NRBITS   = 8
	_IOC_TYPEBITS = 8
	_IOC_SIZEBITS = 14
	// _IOC_DIRBITS = 2

	_IOC_NRSHIFT   = 0
	_IOC_TYPESHIFT = _IOC_NRSHIFT + _IOC_NRBITS
	_IOC_SIZESHIFT = _IOC_TYPESHIFT + _IOC_TYPEBITS
	_IOC_DIRSHIFT  = _IOC_SIZESHIFT + _IOC_SIZEBITS
)
