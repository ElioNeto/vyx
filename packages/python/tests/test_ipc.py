"""Tests for ipc.py module."""
import struct
import msgpack
import pytest
from unittest.mock import AsyncMock, patch, MagicMock

from vyx.ipc import IPCClient, Message, MessageType, ERR_NOT_CONNECTED, start_worker


class TestMessageType:
    """Test MessageType constants."""
    
    def test_type_handshake(self):
        assert MessageType.TYPE_HANDSHAKE == 0x01
    
    def test_type_request(self):
        assert MessageType.TYPE_REQUEST == 0x02
    
    def test_type_response(self):
        assert MessageType.TYPE_RESPONSE == 0x03
    
    def test_type_heartbeat(self):
        assert MessageType.TYPE_HEARTBEAT == 0x04
    
    def test_type_error(self):
        assert MessageType.TYPE_ERROR == 0x05


class TestMessage:
    """Test Message class."""
    
    def test_message_creation(self):
        msg = Message(0x02, b"payload")
        assert msg.type == 0x02
        assert msg.payload == b"payload"
    
    def test_message_empty_payload(self):
        msg = Message(0x01, b"")
        assert msg.type == 0x01
        assert msg.payload == b""


class TestIPCClientInit:
    """Test IPCClient initialization."""
    
    def test_init_with_socket_path(self):
        client = IPCClient(socket_path="/tmp/test.sock")
        assert client.socket_path == "/tmp/test.sock"
        assert client.reader is None
        assert client.writer is None
        assert client.worker_id == ""
        assert client.correlation_id == ""
    
    def test_init_without_socket_path(self):
        import os
        # Clear env var if set
        with patch.dict(os.environ, {}, clear=True):
            client = IPCClient()
            assert client.socket_path is None
    
    def test_init_with_env_socket(self):
        import os
        with patch.dict(os.environ, {"VYX_SOCKET": "/env/test.sock"}):
            client = IPCClient()
            assert client.socket_path == "/env/test.sock"


class TestIPCClientErrors:
    """Test IPCClient error cases."""
    
    def test_write_not_connected(self):
        client = IPCClient(socket_path="/tmp/test.sock")
        msg = Message(0x02, b"test")
        with pytest.raises(RuntimeError, match=ERR_NOT_CONNECTED):
            import asyncio
            asyncio.run(client._write(msg))
    
    def test_read_not_connected(self):
        client = IPCClient(socket_path="/tmp/test.sock")
        with pytest.raises(RuntimeError, match=ERR_NOT_CONNECTED):
            import asyncio
            asyncio.run(client._read())
    
    def test_send_request_not_connected(self):
        client = IPCClient(socket_path="/tmp/test.sock")
        with pytest.raises(RuntimeError, match=ERR_NOT_CONNECTED):
            import asyncio
            asyncio.run(client.send_request("GET", "/test"))
    
    def test_connect_no_socket_path(self):
        client = IPCClient()
        with pytest.raises(RuntimeError, match="VYX_SOCKET not set"):
            import asyncio
            asyncio.run(client.connect())


class TestIPCClientMethods:
    """Test IPCClient methods with mocked connection."""
    
    @pytest.mark.asyncio
    async def test_write_with_mock(self):
        """Test _write with mocked writer."""
        client = IPCClient(socket_path="/tmp/test.sock")
        
        # Create mock writer
        mock_writer = AsyncMock()
        client.writer = mock_writer
        
        msg = Message(0x02, b"test payload")
        await client._write(msg)
        
        # Verify writer.write was called
        assert mock_writer.write.called
        assert mock_writer.drain.called
    
    @pytest.mark.asyncio
    async def test_read_with_mock(self):
        """Test _read with mocked reader."""
        client = IPCClient(socket_path="/tmp/test.sock")
        
        # Create mock reader that returns proper header + payload
        mock_reader = AsyncMock()
        # Header: payload_len=13 (little-endian), msg_type=0x02
        header = struct.pack("<IB", 13, 0x02)
        mock_reader.read.side_effect = [header, b"test payload"]
        client.reader = mock_reader
        
        msg = await client._read()
        assert msg.type == 0x02
        assert msg.payload == b"test payload"
    
    @pytest.mark.asyncio
    async def test_read_connection_closed(self):
        """Test _read when connection is closed."""
        client = IPCClient(socket_path="/tmp/test.sock")
        
        mock_reader = AsyncMock()
        mock_reader.read.return_value = b""  # Empty = closed
        client.reader = mock_reader
        
        with pytest.raises(RuntimeError, match="connection closed"):
            await client._read()
    
    @pytest.mark.asyncio
    async def test_close(self):
        """Test close method."""
        client = IPCClient(socket_path="/tmp/test.sock")
        
        # Mock writer with async wait_closed
        mock_writer = MagicMock()
        mock_writer.wait_closed = AsyncMock()
        client.writer = mock_writer
        
        await client.close()
        assert mock_writer.close.called
        # Note: close() doesn't reset writer/reader to None in current impl

    @pytest.mark.asyncio
    async def test_connect_success(self):
        """Test successful connection."""
        client = IPCClient(socket_path="/tmp/test.sock")
        
        # Mock open_unix_connection
        mock_reader = MagicMock()
        mock_writer = MagicMock()
        
        with patch("asyncio.open_unix_connection", return_value=(mock_reader, mock_writer)) as mock_connect:
            with patch.object(client, '_send_handshake', new_callable=AsyncMock) as mock_handshake:
                await client.connect()
        
        assert client.reader == mock_reader
        assert client.writer == mock_writer
        mock_handshake.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_connect_no_socket_path(self):
        """Test connect without socket path."""
        client = IPCClient()  # No socket path
        
        with pytest.raises(RuntimeError, match="VYX_SOCKET not set"):
            await client.connect()
    
    @pytest.mark.asyncio
    async def test_send_handshake(self):
        """Test _send_handshake method."""
        client = IPCClient(socket_path="/tmp/test.sock")
        client.worker_id = "python:test"
        client.writer = AsyncMock()
        
        with patch.object(client, '_write', new_callable=AsyncMock) as mock_write:
            await client._send_handshake()
        
        mock_write.assert_called_once()
        # Verify the message passed to _write
        call_args = mock_write.call_args[0][0]
        assert call_args.type == MessageType.TYPE_HANDSHAKE
        handshake_data = msgpack.unpackb(call_args.payload, raw=False)
        assert handshake_data["worker_id"] == "python:test"
        assert handshake_data["runtime"] == "python"
    
    @pytest.mark.asyncio
    async def test_send_request(self):
        """Test send_request method."""
        client = IPCClient(socket_path="/tmp/test.sock")
        client.writer = AsyncMock()
        client.correlation_id = "test-corr-id"
        
        # Mock _write and _read
        with patch.object(client, '_write', new_callable=AsyncMock) as mock_write:
            with patch.object(client, '_read', new_callable=AsyncMock) as mock_read:
                # Mock response
                mock_read.return_value = Message(
                    MessageType.TYPE_RESPONSE,
                    msgpack.packb({"status": "ok"}, use_bin_type=True)
                )
                result = await client.send_request("GET", "/test", correlation_id="test-corr-id")
        
        mock_write.assert_called_once()
        assert result == {"status": "ok"}
    
    @pytest.mark.asyncio
    async def test_send_request_error_response(self):
        """Test send_request with error response."""
        client = IPCClient(socket_path="/tmp/test.sock")
        client.writer = AsyncMock()
        
        with patch.object(client, '_write', new_callable=AsyncMock):
            with patch.object(client, '_read', new_callable=AsyncMock) as mock_read:
                mock_read.return_value = Message(
                    MessageType.TYPE_ERROR,
                    msgpack.packb(b"error occurred", use_bin_type=True)
                )
                with pytest.raises(RuntimeError, match="IPC error"):
                    await client.send_request("GET", "/test")
    
    @pytest.mark.asyncio
    async def test_send_request_wrong_response_type(self):
        """Test send_request with unexpected response type."""
        client = IPCClient(socket_path="/tmp/test.sock")
        client.writer = AsyncMock()
        
        with patch.object(client, '_write', new_callable=AsyncMock):
            with patch.object(client, '_read', new_callable=AsyncMock) as mock_read:
                mock_read.return_value = Message(
                    0x99,  # Unknown type
                    b"data"
                )
                with pytest.raises(RuntimeError, match="unexpected message type"):
                    await client.send_request("GET", "/test")
    
    @pytest.mark.asyncio
    async def test_receive_heartbeat(self):
        """Test receive method with heartbeat."""
        client = IPCClient(socket_path="/tmp/test.sock")
        client.reader = AsyncMock()
        
        with patch.object(client, '_read', new_callable=AsyncMock) as mock_read:
            mock_read.return_value = Message(MessageType.TYPE_HEARTBEAT, b"")
            result = await client.receive()
        
        assert result == {"type": "heartbeat"}
    
    @pytest.mark.asyncio
    async def test_receive_request(self):
        """Test receive method with request."""
        client = IPCClient(socket_path="/tmp/test.sock")
        
        test_request = {
            "method": "GET",
            "path": "/test",
            "headers": {},
        }
        
        with patch.object(client, '_read', new_callable=AsyncMock) as mock_read:
            mock_read.return_value = Message(
                MessageType.TYPE_REQUEST,
                msgpack.packb(test_request, use_bin_type=True)
            )
            result = await client.receive()
        
        assert result["method"] == "GET"
        assert result["path"] == "/test"
    
    @pytest.mark.asyncio
    async def test_receive_unexpected_type(self):
        """Test receive with unexpected message type."""
        client = IPCClient(socket_path="/tmp/test.sock")
        
        with patch.object(client, '_read', new_callable=AsyncMock) as mock_read:
            mock_read.return_value = Message(0x99, b"data")
            with pytest.raises(RuntimeError, match="unexpected message type"):
                await client.receive()
    
    @pytest.mark.asyncio
    async def test_send_response(self):
        """Test send_response method."""
        client = IPCClient(socket_path="/tmp/test.sock")
        client.writer = AsyncMock()
        
        with patch.object(client, '_write', new_callable=AsyncMock) as mock_write:
            await client.send_response(200, {"data": "test"}, correlation_id="corr-123")
        
        mock_write.assert_called_once()
        call_args = mock_write.call_args[0][0]
        assert call_args.type == MessageType.TYPE_RESPONSE
        response_data = msgpack.unpackb(call_args.payload, raw=False)
        assert response_data["status_code"] == 200
        assert response_data["correlation_id"] == "corr-123"
    
    @pytest.mark.asyncio
    async def test_send_error(self):
        """Test send_error method."""
        client = IPCClient(socket_path="/tmp/test.sock")
        client.writer = AsyncMock()
        
        with patch.object(client, '_write', new_callable=AsyncMock) as mock_write:
            await client.send_error(500, "Internal error", correlation_id="corr-456")
        
        mock_write.assert_called_once()
        call_args = mock_write.call_args[0][0]
        assert call_args.type == MessageType.TYPE_ERROR
        error_data = msgpack.unpackb(call_args.payload, raw=False)
        assert error_data["status_code"] == 500
        assert error_data["error"] == "Internal error"


class TestStartWorker:
    """Test start_worker function."""
    
    @pytest.mark.asyncio
    async def test_start_worker_handles_requests(self):
        """Test start_worker processes requests."""
        import asyncio
        
        handlers = {
            ("GET", "/test"): lambda req: {"message": "hello"}
        }
        
        # Mock IPCClient
        mock_client = AsyncMock()
        # Return a valid request first, then raise to exit loop
        mock_client.receive.side_effect = [
            {
                "method": "GET",
                "path": "/test",
                "correlation_id": "test-123"
            },
            RuntimeError("exit loop")
        ]
        
        with patch("vyx.ipc.IPCClient", return_value=mock_client):
            try:
                await start_worker(handlers)
            except RuntimeError:
                pass  # Expected
        
        mock_client.connect.assert_called_once()
        mock_client.send_response.assert_called_once()
        call_args = mock_client.send_response.call_args
        assert call_args[0][0] == 200  # status_code
    
    @pytest.mark.asyncio
    async def test_start_worker_handles_heartbeat(self):
        """Test start_worker skips heartbeat."""
        import asyncio
        
        handlers = {}
        
        mock_client = AsyncMock()
        # First return heartbeat, then raise to exit
        mock_client.receive.side_effect = [
            {"type": "heartbeat"},
            RuntimeError("exit loop")
        ]
        
        with patch("vyx.ipc.IPCClient", return_value=mock_client):
            try:
                await start_worker(handlers)
            except RuntimeError:
                pass
        
        # Should NOT call handlers or send_response for heartbeat
        mock_client.send_response.assert_not_called()
        mock_client.send_error.assert_not_called()
    
    @pytest.mark.asyncio
    async def test_start_worker_no_handler(self):
        """Test start_worker with no matching handler."""
        handlers = {}
        
        mock_client = AsyncMock()
        mock_client.receive.side_effect = [
            {
                "method": "GET",
                "path": "/unknown",
                "correlation_id": "test-456"
            },
            RuntimeError("exit loop")
        ]
        
        with patch("vyx.ipc.IPCClient", return_value=mock_client):
            try:
                await start_worker(handlers)
            except RuntimeError:
                pass
        
        mock_client.send_error.assert_called_once()
        call_args = mock_client.send_error.call_args
        assert call_args[0][0] == 404
    
    @pytest.mark.asyncio
    async def test_start_worker_handler_exception(self):
        """Test start_worker handles handler exceptions."""
        # Use sync handler that raises
        def bad_handler(req):
            raise Exception("boom")
        
        handlers = {
            ("GET", "/error"): bad_handler
        }
        
        mock_client = AsyncMock()
        mock_client.receive.side_effect = [
            {
                "method": "GET",
                "path": "/error",
                "correlation_id": "test-789"
            },
            RuntimeError("exit loop")
        ]
        
        with patch("vyx.ipc.IPCClient", return_value=mock_client):
            try:
                await start_worker(handlers)
            except RuntimeError:
                pass
        
        mock_client.send_error.assert_called_once()
        call_args = mock_client.send_error.call_args
        assert call_args[0][0] == 500
    
    @pytest.mark.asyncio
    async def test_start_worker_exit_on_exception(self):
        """Test start_worker exits on outer exception."""
        handlers = {}
        
        mock_client = AsyncMock()
        # Raise RuntimeError on first receive call
        mock_client.receive.side_effect = RuntimeError("connection lost")
        
        with patch("vyx.ipc.IPCClient", return_value=mock_client):
            await start_worker(handlers)
        
        # Should have called close
        mock_client.close.assert_called_once()


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
