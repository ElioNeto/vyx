//go:build !windows

package shm_test

import (
	"os"
	"testing"

	"github.com/ElioNeto/vyx/core/infrastructure/ipc/shm"
)

func TestNewStore_Empty(t *testing.T) {
	s := shm.NewStore()
	if s == nil {
		t.Fatal("NewStore() returned nil")
	}
}

func TestStore_CreateAndClose(t *testing.T) {
	s := shm.NewStore()
	size := int64(4096)

	reg, desc, err := s.Create("test-worker", "req-1", size)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if reg == nil {
		t.Fatal("Create() returned nil region")
	}
	if desc == nil {
		t.Fatal("Create() returned nil descriptor")
	}
	if desc.Size != size {
		t.Errorf("descriptor.Size = %d, want %d", desc.Size, size)
	}
	if desc.ID != "vyx-test-worker-req-1" {
		t.Errorf("descriptor.ID = %q, want %q", desc.ID, "vyx-test-worker-req-1")
	}
	if len(reg.Data) != int(size) {
		t.Errorf("region.Data has %d bytes, want %d", len(reg.Data), size)
	}

	_ = s.Close(desc.Path)

	if _, err := os.Stat(desc.Path); !os.IsNotExist(err) {
		t.Error("file should have been removed after Close")
	}
}

func TestStore_DataPersistence(t *testing.T) {
	s := shm.NewStore()
	size := int64(4096)

	reg, desc, err := s.Create("test-worker", "req-data", size)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Write data to the mmap'd region
	orig := []byte("persistence check")
	n := copy(reg.Data, orig)

	// Verify data is readable from the same region
	if n != len(orig) {
		t.Errorf("copy wrote %d bytes, want %d", n, len(orig))
	}
	if string(reg.Data[:len(orig)]) != string(orig) {
		t.Errorf("expected %q, got %q", string(orig), string(reg.Data[:len(orig)]))
	}

	_ = s.Close(desc.Path)
}

func TestStore_CloseNonExistent(t *testing.T) {
	s := shm.NewStore()
	err := s.Close("/dev/shm/nonexistent-test-file")
	if err == nil {
		t.Error("expected error when closing non-existent region")
	}
}

func TestStore_CloseAll(t *testing.T) {
	s := shm.NewStore()

	reg1, desc1, err := s.Create("w1", "r1", 1024)
	if err != nil {
		t.Fatalf("Create w1/r1 error = %v", err)
	}
	_ = reg1

	reg2, desc2, err := s.Create("w2", "r2", 2048)
	if err != nil {
		t.Fatalf("Create w2/r2 error = %v", err)
	}
	_ = reg2

	if err := s.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}

	if _, err := os.Stat(desc1.Path); !os.IsNotExist(err) {
		t.Error("file from desc1 should have been removed")
	}
	if _, err := os.Stat(desc2.Path); !os.IsNotExist(err) {
		t.Error("file from desc2 should have been removed")
	}
}

func TestStore_CloseAllEmpty(t *testing.T) {
	s := shm.NewStore()
	if err := s.CloseAll(); err != nil {
		t.Errorf("CloseAll() on empty store should not error, got: %v", err)
	}
}

func TestStore_OpenNonExistent(t *testing.T) {
	s := shm.NewStore()
	_, err := s.Open("/dev/shm/vyx-nonexistent-file")
	if err == nil {
		t.Error("expected error when opening non-existent file")
	}
}

func TestStore_CreateMultipleRegions(t *testing.T) {
	s := shm.NewStore()
	ids := []string{"r1", "r2", "r3"}
	var paths []string

	for _, id := range ids {
		_, desc, err := s.Create("multi", id, 1024)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", id, err)
		}
		paths = append(paths, desc.Path)
	}

	for _, p := range paths {
		if err := s.Close(p); err != nil {
			t.Errorf("Close(%q) error = %v", p, err)
		}
	}
}

func TestDescriptor_Fields(t *testing.T) {
	desc := &shm.Descriptor{
		Path: "/dev/shm/vyx-w-id",
		Size: 8192,
		ID:   "vyx-w-id",
	}
	if desc.Path != "/dev/shm/vyx-w-id" {
		t.Errorf("Path = %q", desc.Path)
	}
	if desc.Size != 8192 {
		t.Errorf("Size = %d", desc.Size)
	}
	if desc.ID != "vyx-w-id" {
		t.Errorf("ID = %q", desc.ID)
	}
}
