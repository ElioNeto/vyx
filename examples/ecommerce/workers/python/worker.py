"""
Python worker for the ecommerce vyx example.

Handles payment processing.

Wire protocol (matches core/infrastructure/ipc/framing/framing.go):
  [Length: 4 bytes LE][Type: 1 byte][Payload: N bytes]

Annotated routes (parsed at build time by `vyx build`):

# @Route(POST /api/payments/process)
# @Auth(roles: ["user", "admin"])

# @Route(GET /api/payments/:id)
# @Auth(roles: ["user", "admin"])
"""

import asyncio
import json
import struct
import sys
import time
from datetime import datetime, timezone

# ─── IPC protocol constants ───────────────────────────────────────────────────
TYPE_REQUEST = 0x01
TYPE_RESPONSE = 0x02
TYPE_HEARTBEAT = 0x03
TYPE_HANDSHAKE = 0x05

# ─── In-memory store ──────────────────────────────────────────────────────────
payments = {}
next_payment_id = 5000


def write_frame(writer, msg_type, payload):
    """Write a framed message: [4-byte LE length][1-byte type][payload]."""
    payload_bytes = json.dumps(payload).encode("utf-8") if payload is not None else b""
    header = struct.pack("<I", len(payload_bytes)) + struct.pack("B", msg_type)
    writer.write(header + payload_bytes)


async def read_full(reader, n):
    """Read exactly n bytes from the reader."""
    buf = b""
    while len(buf) < n:
        chunk = await reader.read(n - len(buf))
        if not chunk:
            raise ConnectionError("connection closed")
        buf += chunk
    return buf


async def read_frame(reader):
    """Read a framed message from the reader."""
    header = await read_full(reader, 5)
    length = struct.unpack("<I", header[:4])[0]
    msg_type = header[4]
    payload = b""
    if length > 0:
        payload = await read_full(reader, length)
    return msg_type, payload


# ─── Route handlers ──────────────────────────────────────────────────────────

def get_user_id(claims):
    """Extract user ID from claims."""
    if claims and "sub" in claims:
        return str(claims["sub"])
    return None


def json_response(status_code, body):
    """Build a JSON response."""
    return {
        "status_code": status_code,
        "headers": {"Content-Type": "application/json"},
        "body": body,
    }


# @Route(POST /api/payments/process)
# @Auth(roles: ["user", "admin"])
def handle_process_payment(req):
    """Process a payment for an order."""
    claims = req.get("claims", {})
    user_id = get_user_id(claims)
    if not user_id:
        return json_response(401, {"error": "unauthorized"})

    body = req.get("body", {})
    order_id = body.get("order_id")
    amount = body.get("amount")
    payment_method = body.get("payment_method", "credit_card")

    if not order_id or not amount:
        return json_response(400, {"error": "order_id and amount are required"})

    global next_payment_id
    payment_id = str(next_payment_id)
    next_payment_id += 1

    now = datetime.now(timezone.utc).isoformat()
    payment = {
        "id": payment_id,
        "order_id": order_id,
        "amount": amount,
        "payment_method": payment_method,
        "user_id": user_id,
        "status": "approved",
        "transaction_id": f"txn_{payment_id}_{int(time.time())}",
        "processed_at": now,
    }
    payments[payment_id] = payment

    return json_response(201, payment)


# @Route(GET /api/payments/:id)
# @Auth(roles: ["user", "admin"])
def handle_get_payment(req):
    """Get payment details by ID."""
    claims = req.get("claims", {})
    user_id = get_user_id(claims)
    if not user_id:
        return json_response(401, {"error": "unauthorized"})

    params = req.get("params", {})
    payment_id = params.get("id")

    if not payment_id:
        return json_response(400, {"error": "payment id is required"})

    payment = payments.get(payment_id)
    if not payment:
        return json_response(404, {"error": f"Payment {payment_id} not found"})

    # Users can only see their own payments, admins can see all
    roles = claims.get("roles", [])
    is_admin = "admin" in roles
    if not is_admin and payment.get("user_id") != user_id:
        return json_response(403, {"error": "forbidden"})

    return json_response(200, payment)


def dispatch(req):
    """Route the request to the appropriate handler."""
    method = req.get("method", "")
    path = req.get("path", "")

    if method == "POST" and path == "/api/payments/process":
        return handle_process_payment(req)
    if method == "GET" and path.startswith("/api/payments/"):
        return handle_get_payment(req)

    return json_response(404, {"error": "route not found"})


# ─── Main ────────────────────────────────────────────────────────────────────

async def main():
    # Determine socket path
    default_socket = "/tmp/vyx/python:payments.sock"
    if sys.platform == "win32":
        default_socket = r"\\.\pipe\vyx-python:payments"

    socket_path = default_socket
    args = sys.argv[1:]
    for i in range(len(args) - 1):
        if args[i] == "--vyx-socket":
            socket_path = args[i + 1]

    print(f"[python:payments] connecting to {socket_path}")

    if sys.platform == "win32":
        # On Windows, connect to a named pipe
        reader, writer = await asyncio.open_connection(
            pipe=socket_path
        )
    else:
        reader, writer = await asyncio.open_unix_connection(socket_path)

    print("[python:payments] connected to core")

    # Send handshake
    handshake = {
        "worker_id": "python:payments",
        "capabilities": [
            {"path": "/api/payments/process", "method": "POST"},
            {"path": "/api/payments/:id", "method": "GET"},
        ],
    }
    write_frame(writer, TYPE_HANDSHAKE, handshake)
    await writer.drain()
    print("[python:payments] handshake sent")

    # Send an immediate heartbeat
    write_frame(writer, TYPE_HEARTBEAT, None)
    await writer.drain()
    print("[python:payments] initial heartbeat sent")

    # Main loop
    try:
        while True:
            msg_type, payload = await read_frame(reader)

            if msg_type == TYPE_HEARTBEAT:
                write_frame(writer, TYPE_HEARTBEAT, None)
                await writer.drain()

            elif msg_type == TYPE_REQUEST:
                req = json.loads(payload.decode("utf-8"))
                print(f"[python:payments] {req.get('method')} {req.get('path')}")
                resp = dispatch(req)
                write_frame(writer, TYPE_RESPONSE, resp)
                await writer.drain()

    except (ConnectionError, asyncio.IncompleteReadError) as e:
        print(f"[python:payments] connection closed: {e}")
    finally:
        writer.close()
        await writer.wait_closed()


if __name__ == "__main__":
    asyncio.run(main())
