"""
Python worker for the dashboard vyx example — Analytics API.

Connects to the vyx core via Unix Domain Socket, performs the handshake,
and handles analytics requests. Uses only the Python standard library.

Wire protocol:
  [Length: 4 bytes LE][Type: 1 byte][Payload: JSON bytes]

Annotated routes (parsed at build time by `vyx build`):

# @Route(GET /api/analytics/overview)
# @Auth(roles: ["admin", "user"])

# @Route(GET /api/analytics/revenue)
# @Auth(roles: ["admin"])

# @Route(GET /api/analytics/users/growth)
# @Auth(roles: ["admin", "user"])
"""

import asyncio
import json
import logging
import signal
import struct
import sys

# ─── IPC protocol constants ───────────────────────────────────────────────────
TYPE_REQUEST = 0x01
TYPE_RESPONSE = 0x02
TYPE_HEARTBEAT = 0x03
TYPE_ERROR = 0x04
TYPE_HANDSHAKE = 0x05

logger = logging.getLogger("python:analytics")


# ─── Helpers ──────────────────────────────────────────────────────────────────

def write_frame(writer, msg_type, payload=None):
    """Write a binary frame: [4 bytes LE length][1 byte type][payload]."""
    payload_bytes = json.dumps(payload).encode("utf-8") if payload is not None else b""
    header = struct.pack("<I", len(payload_bytes)) + bytes([msg_type])
    writer.write(header + payload_bytes)


async def read_exact(reader, n):
    """Read exactly n bytes from the reader."""
    buf = b""
    while len(buf) < n:
        chunk = await reader.read(n - len(buf))
        if not chunk:
            raise ConnectionError("connection closed while reading")
        buf += chunk
    return buf


async def read_frame(reader):
    """Read a binary frame from the reader."""
    header = await read_exact(reader, 5)
    length = struct.unpack("<I", header[:4])[0]
    msg_type = header[4]
    if length > 0:
        payload = await read_exact(reader, length)
        return msg_type, json.loads(payload.decode("utf-8"))
    return msg_type, None


# ─── Analytics data ───────────────────────────────────────────────────────────

def get_overview():
    """Return aggregated analytics overview."""
    return {
        "total_users": 1250,
        "total_sessions": 84720,
        "active_users": 438,
        "page_views": 312450,
        "conversion_rate": 3.42,
    }


def get_revenue():
    """Return revenue data (admin only)."""
    return {
        "daily": [
            {"date": "2026-05-21", "revenue": 1520.50},
            {"date": "2026-05-22", "revenue": 1840.75},
            {"date": "2026-05-23", "revenue": 1230.00},
            {"date": "2026-05-24", "revenue": 2100.25},
            {"date": "2026-05-25", "revenue": 1950.80},
            {"date": "2026-05-26", "revenue": 2340.60},
            {"date": "2026-05-27", "revenue": 1780.90},
        ],
        "monthly": [
            {"month": "2026-01", "revenue": 45000.00},
            {"month": "2026-02", "revenue": 52000.00},
            {"month": "2026-03", "revenue": 48000.00},
            {"month": "2026-04", "revenue": 61000.00},
            {"month": "2026-05", "revenue": 58000.00},
        ],
        "total": 99999.99,
        "growth_percent": 12.5,
    }


def get_user_growth():
    """Return user growth data."""
    return {
        "labels": [
            "Jan", "Feb", "Mar", "Apr", "May", "Jun",
            "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
        ],
        "values": [100, 200, 350, 500, 680, 850, 950, 1050, 1100, 1150, 1200, 1250],
        "total": 1250,
    }


# ─── Route handlers ───────────────────────────────────────────────────────────

# @Route(GET /api/analytics/overview)
# @Auth(roles: ["admin", "user"])
def handle_overview(request):
    return {
        "status_code": 200,
        "headers": {"Content-Type": "application/json"},
        "body": get_overview(),
    }


# @Route(GET /api/analytics/revenue)
# @Auth(roles: ["admin"])
def handle_revenue(request):
    return {
        "status_code": 200,
        "headers": {"Content-Type": "application/json"},
        "body": get_revenue(),
    }


# @Route(GET /api/analytics/users/growth)
# @Auth(roles: ["admin", "user"])
def handle_user_growth(request):
    return {
        "status_code": 200,
        "headers": {"Content-Type": "application/json"},
        "body": get_user_growth(),
    }


def dispatch(request):
    """Route the request to the appropriate handler."""
    method = request.get("method", "")
    path = request.get("path", "").rstrip("/")

    if method == "GET" and path == "/api/analytics/overview":
        return handle_overview(request)
    if method == "GET" and path == "/api/analytics/revenue":
        return handle_revenue(request)
    if method == "GET" and path == "/api/analytics/users/growth":
        return handle_user_growth(request)

    return {
        "status_code": 404,
        "headers": {"Content-Type": "application/json"},
        "body": {"error": "route not found"},
    }


# ─── Main connection handler ─────────────────────────────────────────────────

async def handle_connection(reader, writer):
    """Handle messages from the core."""
    logger.info("connected to core")

    # Send handshake — no "type" field, just worker_id + capabilities
    handshake = {
        "worker_id": "python:analytics",
        "capabilities": [
            {"path": "/api/analytics/overview", "method": "GET"},
            {"path": "/api/analytics/revenue", "method": "GET"},
            {"path": "/api/analytics/users/growth", "method": "GET"},
        ],
    }
    write_frame(writer, TYPE_HANDSHAKE, handshake)
    await writer.drain()
    logger.info("handshake sent")

    # Send initial heartbeat
    write_frame(writer, TYPE_HEARTBEAT, None)
    await writer.drain()
    logger.info("initial heartbeat sent")

    try:
        while True:
            msg_type, payload = await read_frame(reader)

            if msg_type == TYPE_HEARTBEAT:
                # Echo heartbeat back
                write_frame(writer, TYPE_HEARTBEAT, None)
                await writer.drain()

            elif msg_type == TYPE_REQUEST:
                method = payload.get("method", "")
                path = payload.get("path", "")
                logger.info("%s %s", method, path)

                response = dispatch(payload)
                write_frame(writer, TYPE_RESPONSE, response)
                await writer.drain()

            elif msg_type == TYPE_ERROR:
                logger.error("error from core: %s", payload)

    except (ConnectionError, asyncio.IncompleteReadError) as exc:
        logger.info("connection closed: %s", exc)
    finally:
        writer.close()
        await writer.wait_closed()


async def main():
    """Entry point."""
    default_socket = "/tmp/vyx/python:analytics.sock"
    socket_path = default_socket

    for i, arg in enumerate(sys.argv[1:]):
        if arg == "--vyx-socket" and i + 1 < len(sys.argv[1:]):
            socket_path = sys.argv[i + 2]
            break

    logger.info("connecting to %s", socket_path)

    reader, writer = await asyncio.open_unix_connection(socket_path)
    await handle_connection(reader, writer)


if __name__ == "__main__":
    logging.basicConfig(
        level=logging.INFO,
        format="[python:analytics] %(message)s",
        stream=sys.stdout,
    )

    # Handle SIGTERM/SIGINT gracefully
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)

    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, lambda: loop.stop())

    try:
        loop.run_until_complete(main())
    except KeyboardInterrupt:
        pass
    finally:
        loop.close()
