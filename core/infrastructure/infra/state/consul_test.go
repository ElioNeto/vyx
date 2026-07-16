package state

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ElioNeto/vyx/core/domain/infra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to extract consul KV path from URL
func consulPath(url string) string {
	prefix := "/v1/kv/"
	if idx := strings.Index(url, prefix); idx >= 0 {
		return url[idx+len(prefix):]
	}
	return url
}

func TestConsulBackend_Init(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v1/status/leader") {
			w.Write([]byte(`"127.0.0.1:8300"`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	be, err := NewConsulBackend(ConsulBackendConfig{Addr: srv.URL})
	require.NoError(t, err)

	err = be.Init(context.Background())
	require.NoError(t, err)
}

func TestConsulBackend_Init_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	be, err := NewConsulBackend(ConsulBackendConfig{Addr: srv.URL})
	require.NoError(t, err)

	err = be.Init(context.Background())
	assert.Error(t, err)
}

func TestConsulBackend_PutGet(t *testing.T) {
	var storedValue []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if strings.Contains(r.URL.RawQuery, "raw") {
				if storedValue == nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.Write(storedValue)
			} else {
				// List request (not used in Get, but for safety)
				w.Write([]byte(`[]`))
			}
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			storedValue = body
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`true`))
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	be, err := NewConsulBackend(ConsulBackendConfig{Addr: srv.URL, Path: "vyx/test"})
	require.NoError(t, err)

	// Put
	state := infra.NewState("test")
	state.Resources = append(state.Resources, infra.NewResource("r1", "mock", "mock"))
	err = be.Put(ctx, state)
	require.NoError(t, err)

	// Get
	got, err := be.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got.Resources, 1)
}

func TestConsulBackend_Get_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	be, err := NewConsulBackend(ConsulBackendConfig{Addr: srv.URL})
	require.NoError(t, err)

	state, err := be.Get(context.Background())
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestConsulBackend_Delete(t *testing.T) {
	var deleted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	be, err := NewConsulBackend(ConsulBackendConfig{Addr: srv.URL})
	require.NoError(t, err)

	err = be.Delete(context.Background())
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestConsulBackend_LockUnlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "acquire=") {
			// First lock succeeds, subsequent fail
			w.Write([]byte(`true`))
			return
		}
		if strings.Contains(r.URL.RawQuery, "release=") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`true`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := context.Background()
	be, err := NewConsulBackend(ConsulBackendConfig{Addr: srv.URL})
	require.NoError(t, err)

	info := infra.LockInfo{ID: "test-lock", Operation: "apply", Who: "tester"}

	err = be.Lock(ctx, info)
	require.NoError(t, err)

	err = be.Unlock(ctx, info)
	require.NoError(t, err)
}
