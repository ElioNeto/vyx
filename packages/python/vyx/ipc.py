import asyncio
import os
import struct
from typing import Any

import msgpack

ERR_NOT_CONNECTED = "not connected"


class MessageType:
    TYPE_HANDSHAKE = 0x01
    TYPE_REQUEST = 0x02
    TYPE_RESPONSE = 0x03
    TYPE_HEARTBEAT = 0x04
    TYPE_ERROR = 0x05


class Message:
    def __init__(self, msg_type: int, payload: bytes):
        self.type = msg_type
        self.payload = payload


class IPCClient:
    """Client for communicating with core via UDS using binary framing."""

    def __init__(self, socket_path: str | None = None):
        self.socket_path = socket_path or os.environ.get("VYX_SOCKET")
        self.reader: asyncio.StreamReader | None = None
        self.writer: asyncio.StreamWriter | None = None
        self.worker_id: str = ""
        self.correlation_id: str = ""

    async def connect(self) -> None:
        """Connect to core's UDS socket."""
        if not self.socket_path:
            raise RuntimeError("VYX_SOCKET not set")

        self.reader, self.writer = await asyncio.open_unix_connection(self.socket_path)

        await self._send_handshake()

    async def _send_handshake(self) -> None:
        """Send handshake to core."""
        handshake = {
            "worker_id": self.worker_id,
            "runtime": "python",
            "version": "0.1.0",
        }
        payload = msgpack.packb(handshake, use_bin_type=True)
        msg = Message(MessageType.TYPE_HANDSHAKE, payload)
        await self._write(msg)

    async def _write(self, msg: Message) -> None:
        """Write a framed message to the socket."""
        if not self.writer:
            raise RuntimeError(ERR_NOT_CONNECTED)

        payload_len = len(msg.payload)
        header = struct.pack("<IB", payload_len, msg.type)

        frame = header + msg.payload
        self.writer.write(frame)
        await self.writer.drain()

    async def _read(self) -> Message:
        """Read a framed message from the socket."""
        if not self.reader:
            raise RuntimeError(ERR_NOT_CONNECTED)

        header = await self.reader.read(5)
        if len(header) < 5:
            raise RuntimeError("connection closed")

        payload_len, msg_type = struct.unpack("<IB", header)
        payload = b""
        if payload_len > 0:
            payload = await self.reader.read(payload_len)

        return Message(msg_type, payload)

    async def send_request(self, method: str, path: str, **kwargs) -> dict:
        """Send a request and wait for response."""
        if not self.writer:
            raise RuntimeError(ERR_NOT_CONNECTED)

        correlation_id = kwargs.get("correlation_id", "")
        request = {
            "method": method,
            "path": path,
            "headers": kwargs.get("headers", {}),
            "query": kwargs.get("query", {}),
            "params": kwargs.get("params", {}),
            "body": kwargs.get("body"),
            "claims": kwargs.get("claims"),
            "correlation_id": correlation_id,
        }
        payload = msgpack.packb(request, use_bin_type=True)
        msg = Message(MessageType.TYPE_REQUEST, payload)
        await self._write(msg)

        response_msg = await self._read()
        if response_msg.type == MessageType.TYPE_ERROR:
            error_data = msgpack.unpackb(response_msg.payload, raw=True)
            raise RuntimeError(f"IPC error: {error_data}")

        if response_msg.type != MessageType.TYPE_RESPONSE:
            raise RuntimeError(f"unexpected message type: {response_msg.type}")

        return msgpack.unpackb(response_msg.payload, raw=False)

    async def receive(self) -> dict:
        """Receive a request from core (blocking)."""
        msg = await self._read()

        if msg.type == MessageType.TYPE_HEARTBEAT:
            return {"type": "heartbeat"}

        if msg.type != MessageType.TYPE_REQUEST:
            raise RuntimeError(f"unexpected message type: {msg.type}")

        return msgpack.unpackb(msg.payload, raw=False)

    async def send_response(self, status_code: int, body: Any, correlation_id: str = "") -> None:
        """Send a response to core."""
        response = {
            "status_code": status_code,
            "headers": {"Content-Type": "application/msgpack"},
            "body": body,
            "correlation_id": correlation_id,
        }
        payload = msgpack.packb(response, use_bin_type=True)
        msg = Message(MessageType.TYPE_RESPONSE, payload)
        await self._write(msg)

    async def send_error(self, status_code: int, error: str, correlation_id: str = "") -> None:
        """Send an error response to core."""
        error_msg = {
            "status_code": status_code,
            "error": error,
            "correlation_id": correlation_id,
        }
        payload = msgpack.packb(error_msg, use_bin_type=True)
        msg = Message(MessageType.TYPE_ERROR, payload)
        await self._write(msg)

    async def close(self) -> None:
        """Close the connection."""
        if self.writer:
            self.writer.close()
            await self.writer.wait_closed()


async def start_worker(handlers: dict):
    """Start the worker process, connecting to core and dispatching requests."""
    client = IPCClient()
    await client.connect()

    while True:
        try:
            request = await client.receive()
            if request.get("type") == "heartbeat":
                continue

            method = request.get("method", "")
            path = request.get("path", "")
            key = (method, path)

            handler = handlers.get(key)
            if not handler:
                await client.send_error(404, f"no handler for {method} {path}")
                continue

            try:
                result = handler(request)
                if asyncio.iscoroutine(result):
                    result = await result
                await client.send_response(200, result, request.get("correlation_id", ""))
            except Exception as e:
                await client.send_error(500, str(e))
        except Exception:
            break

    await client.close()
