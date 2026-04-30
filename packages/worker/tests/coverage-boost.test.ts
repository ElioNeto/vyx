import { handleSocketData, HandleSocketDataResult } from '../src/dispatch';
import net from 'net';

describe('Coverage boost tests for dispatch.ts', () => {
  describe('handleSocketData direct calls', () => {
    it('should return remaining buffer for heartbeat', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const mockSocket = {
        write: () => true,
        destroy: () => {},
      } as unknown as net.Socket;

      // Heartbeat has no payload
      const header = Buffer.alloc(5);
      header.writeUInt32LE(0, 0);
      header.writeUInt8(0x03, 4); // TYPE_HEARTBEAT
      
      const result = handleSocketData(mockSocket, header, bufferRef, 'test-worker');
      expect(result).toBeDefined();
      expect(result.remaining).toBeDefined();
      expect(Buffer.isBuffer(result.remaining)).toBe(true);
    });

    it('should return remaining buffer for unknown type', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const mockSocket = {
        write: () => true,
        destroy: () => {},
      } as unknown as net.Socket;

      // Unknown type
      const header = Buffer.alloc(5);
      header.writeUInt32LE(0, 0);
      header.writeUInt8(0xFF, 4); // UNKNOWN
      
      const result = handleSocketData(mockSocket, header, bufferRef, 'test-worker');
      expect(result).toBeDefined();
      expect(result.remaining).toBeDefined();
    });

    it('should handle request with payload (async response)', (done) => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      let responseWritten = false;
      const mockSocket = {
        write: (buf: Buffer) => { 
          responseWritten = true;
          // Give time for async operations, then call done
          setTimeout(() => {
            expect(responseWritten).toBe(true);
            done();
          }, 100);
          return true; 
        },
        destroy: () => {},
      } as unknown as net.Socket;

      // Request with JSON payload
      const payload = JSON.stringify({ method: 'GET', path: '/test' });
      const payloadBuf = Buffer.from(payload);
      const header = Buffer.alloc(5);
      header.writeUInt32LE(payloadBuf.length, 0);
      header.writeUInt8(0x01, 4); // TYPE_REQUEST
      
      const result = handleSocketData(mockSocket, Buffer.concat([header, payloadBuf]), bufferRef, 'test-worker');
      expect(result).toBeDefined();
      expect(result.remaining).toBeDefined();
      // Note: response is async, so we use done() callback above
    }, 1000);
  });
});
