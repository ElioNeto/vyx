// Package handshake implements the worker registration protocol (spec §8.1).
//
// When a worker process starts, it opens a UDS connection to the core and
// immediately sends a TypeHandshake frame. The HandshakeHandler reads that
// frame, decodes the capability list, cross-validates it against the static
// route_map, logs any mismatches as warnings, and transitions the worker
// state to Running.
//
// Architecture note: Handler is an application-layer use case. It coordinates
// the IPC transport (domain port) and the lifecycle service (application
// service). It has zero knowledge of sockets or OS specifics.
package handshake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"

	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// LifecycleService is the subset of lifecycle.Service used by the handshake handler.
type LifecycleService interface {
	MarkRunning(ctx context.Context, workerID string) error
}

// Transport is the subset of ipc.Transport used here.
type Transport interface {
	Receive(ctx context.Context, workerID string) (ipc.Message, error)
}

// Handler performs the worker registration handshake for a single worker.
type Handler struct {
	transport Transport
	routes    *dgw.RouteMap
	service   LifecycleService
	log       *zap.Logger
}

// NewHandler creates a Handler wired with the required dependencies.
func NewHandler(
	transport Transport,
	routes *dgw.RouteMap,
	service LifecycleService,
	log *zap.Logger,
) *Handler {
	return &Handler{
		transport: transport,
		routes:    routes,
		service:   service,
		log:       log,
	}
}

// Handle waits for a TypeHandshake frame from workerID within ctx deadline.
// It decodes the capability list, validates routes, and calls MarkRunning.
// If the handshake times out or the payload is malformed, the error is
// returned so the caller can kill and restart the worker.
func (h *Handler) Handle(ctx context.Context, workerID string) error {
	msg, err := h.transport.Receive(ctx, workerID)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("handshake: worker %s did not send handshake within deadline: %w",
				workerID, ctx.Err())
		}
		return fmt.Errorf("handshake: receive from worker %s: %w", workerID, err)
	}

	if msg.Type != ipc.TypeHandshake {
		return fmt.Errorf("handshake: worker %s sent unexpected message type %s (want handshake)",
			workerID, msg.Type)
	}

	var payload ipc.HandshakePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("handshake: worker %s sent malformed payload: %w", workerID, err)
	}

	h.validateCapabilities(workerID, payload.Capabilities)

	if err := h.service.MarkRunning(ctx, workerID); err != nil {
		return fmt.Errorf("handshake: MarkRunning for worker %s: %w", workerID, err)
	}

	h.log.Info("worker handshake complete",
		zap.String("worker_id", workerID),
		zap.Int("capabilities", len(payload.Capabilities)),
	)
	return nil
}

// validateCapabilities cross-checks declared routes against the route map.
// Mismatches are logged as warnings — they do not block the worker from starting.
func (h *Handler) validateCapabilities(workerID string, caps []ipc.HandshakeCapability) {
	for _, cap := range caps {
		method := strings.ToUpper(cap.Method)
		_, ok := h.routes.Lookup(method, cap.Path)
		if !ok {
			h.log.Warn("handshake: worker declared route not present in route_map",
				zap.String("worker_id", workerID),
				zap.String("method", method),
				zap.String("path", cap.Path),
			)
		}
	}
}
