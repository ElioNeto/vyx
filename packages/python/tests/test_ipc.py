"""Tests for ipc.py module."""
import struct
import msgpack
import pytest
from unittest.mock import AsyncMock, patch, MagicMock

from vyx.ipc import IPCClient, Message, MessageType, ERR_NOT_CONNECTED


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


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
