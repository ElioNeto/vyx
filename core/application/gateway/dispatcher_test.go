package gateway_test

import (
	"context"

	"github.com/ElioNeto/vyx/core/domain/ipc"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
)

// --- mocks ---

type mockJWT struct {
	claims *dgw.Claims
	err    error
}

func (m *mockJWT) Validate(_ string) (*dgw.Claims, error) { return m.claims, m.err }

type mockSchema struct{ err error }

func (m *mockSchema) Validate(_ string, _ []byte) error { return m.err }

type mockTransport struct {
	sendErr error
	respMsg ipc.Message
	recvErr error
}

func (m *mockTransport) Send(_ context.Context, _ string, _ ipc.Message) error { return m.sendErr }
func (m *mockTransport) Receive(_ context.Context, _ string) (ipc.Message, error) {
	return m.respMsg, m.recvErr
}
func (m *mockTransport) ReceiveResponse(_ context.Context, _ string) (ipc.Message, error) {
	return m.respMsg, m.recvErr
}
func (m *mockTransport) Register(_ context.Context, _ string) error   { return nil }
func (m *mockTransport) Deregister(_ context.Context, _ string) error { return nil }
func (m *mockTransport) Close() error                                  { return nil }

// --- tests ---