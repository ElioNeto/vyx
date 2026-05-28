//go:build !windows

package shm

import (
	"os"

	"golang.org/x/sys/unix"
)

func mmapFile(f *os.File, size int64) ([]byte, error) {
	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func munmap(data []byte) error {
	return unix.Munmap(data)
}
