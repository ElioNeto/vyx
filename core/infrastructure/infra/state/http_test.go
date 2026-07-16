package state

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBackend_Init(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	be := NewHTTPBackend(HTTPBackendConfig{Addr: srv.URL})
	err := be.Init(context.Background())
	require.NoError(t, err)
}

func TestHTTPBackend_Init_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Init calls /health — return 500 to simulate failure
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	be := NewHTTPBackend(HTTPBackendConfig{Addr: srv.URL})
	err := be.Init(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http backend")
}

func TestHTTPBackend_PutGet(t *testing.T) {
	var storedState *infra.State

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var s infra.State
			if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			storedState = &s
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if storedState == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(storedState)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	be := NewHTTPBackend(HTTPBackendConfig{Addr: srv.URL})

	// Put
	state := infra.NewState("test")
	state.Resources = append(state.Resources, infra.NewResource("r1", "mock", "mock"))
	err := be.Put(ctx, state)
	require.NoError(t, err)

	// Get
	got, err := be.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got.Resources, 1)
	assert.Equal(t, infra.ResourceID("r1"), got.Resources[0].ID)
}

func TestHTTPBackend_Get_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	be := NewHTTPBackend(HTTPBackendConfig{Addr: srv.URL})
	state, err := be.Get(context.Background())
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestHTTPBackend_LockUnlock(t *testing.T) {
	locked := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			if !locked {
				locked = true
				w.Write([]byte("true"))
			} else {
				w.Write([]byte("false"))
			}
		case http.MethodDelete:
			locked = false
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	be := NewHTTPBackend(HTTPBackendConfig{Addr: srv.URL})
	info := infra.LockInfo{ID: "test", Operation: "apply", Who: "tester"}

	// Lock
	err := be.Lock(ctx, info)
	require.NoError(t, err)

	// Lock again should fail
	err = be.Lock(ctx, info)
	assert.Error(t, err)

	// Unlock
	err = be.Unlock(ctx, info)
	require.NoError(t, err)

	// Lock again should succeed
	err = be.Lock(ctx, info)
	require.NoError(t, err)

	// Cleanup
	be.Unlock(ctx, info)
}

func TestHTTPBackend_Delete(t *testing.T) {
	var deleted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	be := NewHTTPBackend(HTTPBackendConfig{Addr: srv.URL})
	err := be.Delete(context.Background())
	require.NoError(t, err)
	assert.True(t, deleted)
}
