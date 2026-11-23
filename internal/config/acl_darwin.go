package config

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	answerSize                 = 4096
	answerHeaderSize           = 12
	referenceStart             = 4
	extendedSecurityMagic      = 0x012cc16d
	extendedSecurityHeaderSize = 44
	entryCountStart            = 36
	entrySize                  = 24
	noACL                      = 0xffffffff
)

func ACLEntries(file *os.File) (int, error) {
	attributes := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Commonattr: unix.ATTR_CMN_EXTENDED_SECURITY}
	answer := make([]byte, answerSize)
	_, _, errno := unix.Syscall6(unix.SYS_FGETATTRLIST, file.Fd(), uintptr(unsafe.Pointer(&attributes)), uintptr(unsafe.Pointer(&answer[0])), uintptr(len(answer)), 0, 0)
	if errno != 0 {
		return 0, fmt.Errorf("fgetattrlist: %w", errno)
	}
	return entriesInAnswer(answer)
}

func entriesInAnswer(answer []byte) (int, error) {
	offset := int32(binary.NativeEndian.Uint32(answer[referenceStart:]))
	length := binary.NativeEndian.Uint32(answer[referenceStart+4:])
	if length == 0 {
		return 0, nil
	}
	start := referenceStart + int64(offset)
	end := start + int64(length)
	if start < answerHeaderSize || end > int64(len(answer)) {
		return 0, fmt.Errorf("the extended security of %d bytes at offset %d is outside the answer of %d bytes", length, offset, len(answer))
	}
	security := answer[start:end]
	if len(security) < extendedSecurityHeaderSize || binary.NativeEndian.Uint32(security) != extendedSecurityMagic {
		return 0, errors.New("the extended security does not start with the magic 0x012cc16d and an entry count")
	}
	count := binary.NativeEndian.Uint32(security[entryCountStart:])
	if count == noACL {
		return 0, nil
	}
	if uint64(count)*entrySize > uint64(len(security)-extendedSecurityHeaderSize) {
		return 0, fmt.Errorf("the extended security of %d bytes is too short for its %d entries", len(security), count)
	}
	return int(count), nil
}
