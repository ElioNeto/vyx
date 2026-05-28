// Package shm provides shared-memory (mmap) transport for zero-copy
// transfer of large Arrow payloads between core and workers. #7
//
// Shared-memory files are created under /dev/shm/vyx-<workerID>-<requestID>
// and are automatically cleaned up after transfer.
package shm

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const shmDir = "/dev/shm"

// Descriptor identifies a shared-memory region. The core writes Arrow IPC
// bytes into the file and sends this descriptor over UDS to the worker. #7
type Descriptor struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	ID   string `json:"id"`
}

// Region holds an mmap'd shared-memory segment.
type Region struct {
	Data []byte
	Path string
}

// Store manages shared-memory file creation and cleanup.
type Store struct {
	mu      sync.Mutex
	regions map[string]*Region
}

// NewStore creates a shared-memory store.
func NewStore() *Store {
	return &Store{
		regions: make(map[string]*Region),
	}
}

// Create allocates a shared-memory file of the given size and returns a
// descriptor that can be sent over IPC. The caller should call Close on the
// returned region after use. #7
func (s *Store) Create(workerID, requestID string, size int64) (*Region, *Descriptor, error) {
	if err := os.MkdirAll(shmDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("shm: mkdir: %w", err)
	}

	name := fmt.Sprintf("vyx-%s-%s", workerID, requestID)
	path := filepath.Join(shmDir, name)

	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("shm: create file: %w", err)
	}

	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, nil, fmt.Errorf("shm: truncate: %w", err)
	}

	data, err := mmapFile(f, size)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, nil, fmt.Errorf("shm: mmap: %w", err)
	}

	_ = f.Close()

	reg := &Region{Data: data, Path: path}

	s.mu.Lock()
	s.regions[path] = reg
	s.mu.Unlock()

	desc := &Descriptor{
		Path: path,
		Size: size,
		ID:   name,
	}

	return reg, desc, nil
}

// Open opens an existing shared-memory file and mmaps it for reading. #7
// Used by workers to read data sent via TypeArrowSHM.
func (s *Store) Open(path string) (*Region, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("shm: open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("shm: stat: %w", err)
	}

	data, err := mmapFile(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("shm: mmap: %w", err)
	}

	reg := &Region{Data: data, Path: path}

	s.mu.Lock()
	s.regions[path] = reg
	s.mu.Unlock()

	return reg, nil
}

// Close unmaps the region and removes the shared-memory file.
func (s *Store) Close(path string) error {
	s.mu.Lock()
	reg, ok := s.regions[path]
	delete(s.regions, path)
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("shm: region %s not found", path)
	}

	if err := munmap(reg.Data); err != nil {
		return fmt.Errorf("shm: munmap: %w", err)
	}

	_ = os.Remove(path)
	return nil
}

// CloseAll unmaps and removes all tracked shared-memory regions.
func (s *Store) CloseAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	for path, reg := range s.regions {
		if err := munmap(reg.Data); err != nil {
			lastErr = err
		}
		_ = os.Remove(path)
		delete(s.regions, path)
	}
	return lastErr
}
