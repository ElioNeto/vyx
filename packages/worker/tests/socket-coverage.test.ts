import { describe, it, expect, jest } from '@jest/globals';
import net from 'net';
import {
  handleSocketConnect,
  setupProcessHandlers,
  getSocketPath,
} from '../src/dispatch';

describe('Socket Coverage', () => {
  describe('handleSocketConnect', () => {
    it('should send handshake with capabilities', () => {
      const writeCalls: Array<{ type: number; payload: unknown }> = [];
      const mockSocket = {
        write: (frame: Buffer) => {
          const payload = frame.slice(5);
          const payloadStr = payload.toString();
          if (payloadStr.startsWith('{')) {
            const parsed = JSON.parse(payloadStr);
            writeCalls.push({ type: 0x05, payload: parsed });
          } else {
            writeCalls.push({ type: 0x03, payload: null });
          }
        },
      } as unknown as net.Socket;

      handleSocketConnect(mockSocket, 'test-capabilities', [
        { path: '/api/test', method: 'GET' },
        { path: '/api/users', method: 'POST' },
      ]);

      expect(writeCalls.length).toBe(2);
      expect(writeCalls[0].payload).toEqual({
        type: 'handshake',
        worker_id: 'test-capabilities',
        capabilities: [
          { path: '/api/test', method: 'GET' },
          { path: '/api/users', method: 'POST' },
        ],
      });
    });

    it('should send heartbeat after handshake', () => {
      const mockSocket = {
        write: jest.fn(() => true),
      } as unknown as net.Socket;

      handleSocketConnect(mockSocket, 'test-heartbeat', []);

      expect(mockSocket.write).toHaveBeenCalledTimes(2);
    });

    it('should send empty capabilities when not provided', () => {
      const mockSocket = {
        write: jest.fn(() => true),
      } as unknown as net.Socket;

      handleSocketConnect(mockSocket, 'test-empty');

      const firstCall = mockSocket.write.mock.calls[0][0] as Buffer;
      const payload = firstCall.slice(5);
      const handshake = JSON.parse(payload.toString());
      expect(handshake.capabilities).toEqual([]);
    });
  });

  describe('setupProcessHandlers', () => {
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

    it('should not exit when shouldExit is false', () => {
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

    it('should not set up keepAlive when shouldExit is false', () => {
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

  describe('getSocketPath', () => {
    it('should use provided socketPath', () => {
      const path = getSocketPath({ workerId: 'test', socketPath: '/custom/path.sock' });
      expect(path).toBe('/custom/path.sock');
    });

    it('should generate default path for Linux', () => {
      const originalPlatform = process.platform;
      Object.defineProperty(process, 'platform', { value: 'linux', writable: true });
      
      const path = getSocketPath({ workerId: 'test-worker' });
      expect(path).toContain('.vyx/sockets');
      expect(path).toContain('vyx-test-worker.sock');
      
      Object.defineProperty(process, 'platform', { value: originalPlatform, writable: true });
    });

    it('should generate Windows pipe path', () => {
      const originalPlatform = process.platform;
      Object.defineProperty(process, 'platform', { value: 'win32', writable: true });
      
      const path = getSocketPath({ workerId: 'test-worker' });
      expect(path).toBe('\\\\.\\pipe\\vyx-test-worker');
      
      Object.defineProperty(process, 'platform', { value: originalPlatform, writable: true });
    });
  });
});
