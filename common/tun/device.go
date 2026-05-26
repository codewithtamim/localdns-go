package tun

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func MakeTunDeviceFromFD(fd int) (*os.File, error) {
	if fd < 0 {
		return nil, errors.New("must provide a valid TUN file descriptor")
	}
	// Make a copy of `fd` so that os.File's finalizer doesn't close `fd`.
	newfd, err := unix.Dup(fd)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(newfd), "")
	if file == nil {
		return nil, errors.New("failed to open TUN file descriptor")
	}
	return file, nil
}
