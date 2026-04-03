package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"

	apgw "github.com/ElioNeto/vyx/core/application/gateway"
	dgw "github.com/ElioNeto/vyx/core/domain/gateway"
	"github.com/ElioNeto/vyx/core/domain/ipc"
)

// wsProxy is the WebSocket upgrade + proxying handler (#19).
//
// Pipeline:
//  1. Detect Upgrade: websocket header.
//  2. Lookup route in RouteMap (method=WS).
//  3. Enforce JWT + role auth (same logic as HTTP dispatcher).
//  4. Perform WebSocket handshake with the client.
//  5. Send TypeWSOpen frame to the worker.
//  6. Pump frames bidirectionally until either side closes.
//  7. Send TypeWSClose frame to the worker.
type wsProxy struct {
	routes      *dgw.RouteMap
	transport   ipc.Transport
	jwt         apgw.JWTValidator
	log         *zap.Logger
	timeout     time.Duration
}

func newWSProxy(
	routes *dgw.RouteMap,
	transport ipc.Transport,
	jwt apgw.JWTValidator,
	log *zap.Logger,
	timeout time.Duration,
) *wsProxy {
	return &wsProxy{
		routes:    routes,
		transport: transport,
		jwt:       jwt,
		log:       log,
		timeout:   timeout,
	}
}

// isWebSocketUpgrade returns true if the request is a WebSocket upgrade.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// ServeHTTP implements http.Handler. It enforces auth then proxies the WS.
func (p *wsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Route lookup using synthetic method "WS".
	result, ok := p.routes.Lookup("WS", r.URL.Path)
	if !ok {
		http.Error(w, `{"error":"route not found"}`, http.StatusNotFound)
		return
	}
	route := result.Entry

	// 2. JWT + role auth (pre-upgrade — no cost if rejected).
	var claims *dgw.Claims
	if len(route.AuthRoles) > 0 {
		token := r.Header.Get("Authorization")
		if len(token) > 7 && strings.EqualFold(token[:7], "bearer ") {
			token = token[7:]
		}
		c, err := p.jwt.Validate(token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if !hasWSRole(c.Roles, route.AuthRoles) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		claims = c
	}

	// 3. Perform WebSocket upgrade.
	websocket.Handler(func(conn *websocket.Conn) {
		p.proxy(r.Context(), conn, route, result.Params, claims)
	}).ServeHTTP(w, r)
}

func (p *wsProxy) proxy(
	ctx context.Context,
	conn *websocket.Conn,
	route dgw.RouteEntry,
	params map[string]string,
	claims *dgw.Claims,
) {
	sessionID := uuid.NewString()
	workerID := route.WorkerID

	// Build headers snapshot.
	headers := make(map[string]string)
	for k, vs := range conn.Request().Header {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}

	// 4. Notify worker: session opened.
	openPayload, _ := json.Marshal(ipc.WSOpenPayload{
		SessionID: sessionID,
		Path:      conn.Request().URL.Path,
		Headers:   headers,
		Claims:    claims,
	})
	if err := p.transport.Send(ctx, workerID, ipc.Message{
		Type:    ipc.TypeWSOpen,
		Payload: openPayload,
	}); err != nil {
		p.log.Error("ws: failed to notify worker of open",
			zap.String("session_id", sessionID),
			zap.String("worker_id", workerID),
			zap.Error(err),
		)
		return
	}

	p.log.Info("ws: session opened",
		zap.String("session_id", sessionID),
		zap.String("worker_id", workerID),
		zap.String("path", conn.Request().URL.Path),
	)

	defer func() {
		// 7. Notify worker: session closed.
		closePayload, _ := json.Marshal(ipc.WSClosePayload{
			SessionID: sessionID,
			Code:      1000,
			Reason:    "normal closure",
		})
		_ = p.transport.Send(context.Background(), workerID, ipc.Message{
			Type:    ipc.TypeWSClose,
			Payload: closePayload,
		})
		p.log.Info("ws: session closed",
			zap.String("session_id", sessionID),
		)
	}()

	// 5. Pump client → worker.
	errCh := make(chan error, 2)

	go func() {
		for {
			var data []byte
			if err := websocket.Message.Receive(conn, &data); err != nil {
				if err != io.EOF {
					p.log.Debug("ws: client read error",
						zap.String("session_id", sessionID), zap.Error(err))
				}
				errCh <- err
				return
			}
			msgPayload, _ := json.Marshal(ipc.WSMessagePayload{
				SessionID: sessionID,
				Data:      data,
				IsBinary:  false,
			})
			if err := p.transport.Send(ctx, workerID, ipc.Message{
				Type:    ipc.TypeWSMessage,
				Payload: msgPayload,
			}); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// 6. Pump worker → client.
	go func() {
		for {
			msg, err := p.transport.ReceiveResponse(ctx, workerID)
			if err != nil {
				errCh <- err
				return
			}
			if msg.Type != ipc.TypeWSMessage {
				continue // skip non-ws frames (heartbeats etc.)
			}
			var msgPayload ipc.WSMessagePayload
			if err := json.Unmarshal(msg.Payload, &msgPayload); err != nil {
				continue
			}
			if msgPayload.SessionID != sessionID {
				continue // not ours
			}
			if err := websocket.Message.Send(conn, msgPayload.Data); err != nil {
				errCh <- err
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
	case <-errCh:
	}
}

func hasWSRole(callerRoles, required []string) bool {
	set := make(map[string]struct{}, len(callerRoles))
	for _, r := range callerRoles {
		set[r] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}
