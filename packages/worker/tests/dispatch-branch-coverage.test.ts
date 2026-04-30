import { describe, it, expect, jest } from '@jest/globals';
import net from 'net';
import {
  handleSocketData,
  setupSocketHandlers,
  setupProcessHandlers,
  handleSocketConnect,
  dispatch,
  matchRoute,
  writeFrame,
  parseFrames,
} from '../src/dispatch';

describe('Dispatch Branch Coverage', () => {
  describe('setupProcessHandlers edge cases', () => {
    let mockExit: jest.SpyInstance;
    
    beforeEach(() => {
      process.removeAllListeners('SIGTERM');
      process.removeAllListeners('SIGINT');
      mockExit = jest.spyOn(process, 'exit').mockImplementation(() => undefined as any);
    });
    
    afterEach(() => {
      mockExit.mockRestore();
      process.removeAllListeners('SIGTERM');
      process.removeAllListeners('SIGINT');
    });

    it('should cleanup but not exit when shouldExit is false', () => {
      const mockSocket = {
        destroy: jest.fn(),
      } as unknown as net.Socket;

      setupProcessHandlers(mockSocket, false);

      process.emit('SIGTERM');
      expect(mockSocket.destroy).toHaveBeenCalled();
      expect(process.exit).not.toHaveBeenCalled();
    });

    it('should not exit when VYX_TEST_MODE is set', () => {
      process.env.VYX_TEST_MODE = '1';
      const mockSocket = {
        destroy: jest.fn(),
      } as unknown as net.Socket;

      setupProcessHandlers(mockSocket, true);

      process.emit('SIGTERM');
      expect(mockSocket.destroy).toHaveBeenCalled();
      expect(process.exit).not.toHaveBeenCalled();

      delete process.env.VYX_TEST_MODE;
    });

    it('should exit on SIGTERM when shouldExit is true', () => {
      const mockSocket = {
        destroy: jest.fn(),
      } as unknown as net.Socket;

      setupProcessHandlers(mockSocket, true);

      process.emit('SIGTERM');
      expect(mockSocket.destroy).toHaveBeenCalled();
      expect(process.exit).toHaveBeenCalledWith(0);
    });

    it('should exit on SIGINT when shouldExit is true', () => {
      const mockSocket = {
        destroy: jest.fn(),
      } as unknown as net.Socket;

      setupProcessHandlers(mockSocket, true);

      process.emit('SIGINT');
      expect(mockSocket.destroy).toHaveBeenCalled();
      expect(process.exit).toHaveBeenCalledWith(0);
    });

    it('should not set up keepAlive interval when shouldExit is false', () => {
      const mockSocket = {
        destroy: jest.fn(),
      } as unknown as net.Socket;

      const originalSetInterval = global.setInterval;
      const mockSetInterval = jest.fn(() => 1 as unknown as ReturnType<typeof setInterval>);
      global.setInterval = mockSetInterval;

      setupProcessHandlers(mockSocket, false);

      expect(mockSetInterval).not.toHaveBeenCalled();

      global.setInterval = originalSetInterval;
    });
  });

  describe('handleSocketData edge cases', () => {
    it('should handle empty payload for request', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const mockSocket = {
        write: jest.fn(() => true),
        destroy: jest.fn(),
      } as unknown as net.Socket;

      const header = Buffer.alloc(5);
      header.writeUInt32LE(0, 0);
      header.writeUInt8(0x01, 4);

      const result = handleSocketData(mockSocket, header, bufferRef, 'test-worker');
      expect(result).toBeDefined();
      expect(result.remaining).toBeDefined();
    });

    it('should handle malformed JSON in request', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const mockSocket = {
        write: jest.fn(() => true),
        destroy: jest.fn(),
      } as unknown as net.Socket;

      const malformedJson = Buffer.from('{ invalid json }');
      const header = Buffer.alloc(5);
      header.writeUInt32LE(malformedJson.length, 0);
      header.writeUInt8(0x01, 4);

      const result = handleSocketData(mockSocket, Buffer.concat([header, malformedJson]), bufferRef, 'test-worker');
      expect(result).toBeDefined();
    });

    it('should handle unknown message type', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const mockSocket = {
        write: jest.fn(() => true),
        destroy: jest.fn(),
      } as unknown as net.Socket;

      const header = Buffer.alloc(5);
      header.writeUInt32LE(0, 0);
      header.writeUInt8(0xFF, 4);

      const result = handleSocketData(mockSocket, header, bufferRef, 'test-worker');
      expect(result).toBeDefined();
    });

    it('should handle handshake message type', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const mockSocket = {
        write: jest.fn(() => true),
        destroy: jest.fn(),
      } as unknown as net.Socket;

      const header = Buffer.alloc(5);
      header.writeUInt32LE(0, 0);
      header.writeUInt8(0x05, 4);

      const result = handleSocketData(mockSocket, header, bufferRef, 'test-worker');
      expect(result).toBeDefined();
    });
  });

  describe('handleSocketConnect', () => {
    it('should send handshake and heartbeat', () => {
      const mockSocket = {
        write: jest.fn(() => true),
      } as unknown as net.Socket;

      handleSocketConnect(mockSocket, 'test-worker', [
        { path: '/api/test', method: 'GET' },
      ]);

      expect(mockSocket.write).toHaveBeenCalledTimes(2);
    });

    it('should send empty capabilities when not provided', () => {
      const mockSocket = {
        write: jest.fn(() => true),
      } as unknown as net.Socket;

      handleSocketConnect(mockSocket, 'test-worker');

      const firstCall = mockSocket.write.mock.calls[0][0] as Buffer;
      expect(Buffer.isBuffer(firstCall)).toBe(true);
      
      const payload = firstCall.slice(5);
      const handshake = JSON.parse(payload.toString());
      expect(handshake).toEqual({
        type: 'handshake',
        worker_id: 'test-worker',
        capabilities: [],
      });
    });
  });

  describe('dispatch', () => {
    it('should return 404 for unmatched route', async () => {
      const result = await dispatch({
        method: 'GET',
        path: '/nonexistent',
        headers: {},
        query: {},
        params: {},
        body: null,
        claims: null,
        correlation_id: 'test-id',
      });

      expect(result.status_code).toBe(404);
      expect(result.body).toEqual({ error: 'route not found' });
    });
  });

  describe('matchRoute', () => {
    it('should not match when no routes registered', () => {
      const handler = matchRoute('GET', '/api/users');
      expect(handler).toBeUndefined();
    });

    it('should not match different method', () => {
      const handler = matchRoute('POST', '/api/users');
      expect(handler).toBeUndefined();
    });

    it('should not match different path', () => {
      const handler = matchRoute('GET', '/api/orders');
      expect(handler).toBeUndefined();
    });

    it('should handle path with parameters', () => {
      const handler = matchRoute('GET', '/api/users/123');
      if (handler) {
        const result = handler({
          method: 'GET',
          path: '/api/users/123',
          headers: {},
          query: {},
          params: {},
          body: null,
          claims: null,
        });
        expect(result).toBeDefined();
      }
    });
  });

  describe('writeFrame', () => {
    it('should write frame with payload', () => {
      const mockSocket = {
        write: jest.fn(),
      } as unknown as net.Socket;

      writeFrame(mockSocket, 0x01, { test: 'data' });

      expect(mockSocket.write).toHaveBeenCalled();
      const written = mockSocket.write.mock.calls[0][0] as Buffer;
      expect(written.length).toBeGreaterThan(5);
    });

    it('should write frame without payload', () => {
      const mockSocket = {
        write: jest.fn(),
      } as unknown as net.Socket;

      writeFrame(mockSocket, 0x03, null);

      expect(mockSocket.write).toHaveBeenCalled();
      const written = mockSocket.write.mock.calls[0][0] as Buffer;
      expect(written.length).toBe(5);
    });
  });

  describe('parseFrames', () => {
    it('should parse single frame', () => {
      const payload = Buffer.from(JSON.stringify({ test: 'data' }));
      const header = Buffer.alloc(5);
      header.writeUInt32LE(payload.length, 0);
      header.writeUInt8(0x01, 4);

      const buffer = Buffer.concat([header, payload]);
      const result = parseFrames(buffer);

      expect(result.frames.length).toBe(1);
      expect(result.frames[0].msgType).toBe(0x01);
      expect(result.remaining.length).toBe(0);
    });

    it('should parse multiple frames', () => {
      const payload1 = Buffer.from(JSON.stringify({ test: 'data1' }));
      const header1 = Buffer.alloc(5);
      header1.writeUInt32LE(payload1.length, 0);
      header1.writeUInt8(0x01, 4);

      const payload2 = Buffer.from(JSON.stringify({ test: 'data2' }));
      const header2 = Buffer.alloc(5);
      header2.writeUInt32LE(payload2.length, 0);
      header2.writeUInt8(0x02, 4);

      const buffer = Buffer.concat([header1, payload1, header2, payload2]);
      const result = parseFrames(buffer);

      expect(result.frames.length).toBe(2);
      expect(result.remaining.length).toBe(0);
    });

    it('should return remaining buffer for partial frame', () => {
      const header = Buffer.alloc(5);
      header.writeUInt32LE(100, 0);
      header.writeUInt8(0x01, 4);

      const buffer = header;
      const result = parseFrames(buffer);

      expect(result.frames.length).toBe(0);
      expect(result.remaining.length).toBe(5);
    });

    it('should handle empty buffer', () => {
      const buffer = Buffer.alloc(0);
      const result = parseFrames(buffer);

      expect(result.frames.length).toBe(0);
      expect(result.remaining.length).toBe(0);
    });
  });
});
