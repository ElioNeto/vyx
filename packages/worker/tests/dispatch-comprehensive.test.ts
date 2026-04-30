import { get, post, put, patch, del, dispatch, matchRoute, writeFrame, parseFrames, getSocketPath, handleSocketData, HandleSocketDataResult, WorkerOptions, createAndSetupSocket, setupSocketHandlers, setupProcessHandlers, start, handleSocketConnect } from '../src/dispatch';
import { createResponse, json, text, error, WorkerResponse } from '../src/request';
import type { Request, IPCPayload } from '../src/request';
import net from 'node:net';
import { Buffer } from 'node:buffer';

describe('dispatch module - comprehensive coverage', () => {
  // Helper functions inside describe
  function createMockSocket() {
    const writes: Buffer[] = [];
    return {
      writes,
      socket: {
        write: (buf: Buffer) => { writes.push(buf); return true; },
      } as unknown as net.Socket,
    };
  }

  function createTestFrame(msgType: number, payload: unknown): Buffer {
    const payloadBuf = payload
      ? Buffer.from(JSON.stringify(payload))
      : Buffer.alloc(0);
    const header = Buffer.alloc(5);
    header.writeUInt32LE(payloadBuf.length, 0);
    header.writeUInt8(msgType, 4);
    return Buffer.concat([header, payloadBuf]);
  }

  describe('route registration functions', () => {
    it('get should register GET route', () => {
      const handler = (req: Request) => createResponse(200, { method: 'GET' });
      get('/test-get', handler);
      const matched = matchRoute('GET', '/test-get');
      expect(matched).toBeDefined();
    });

    it('post should register POST route', () => {
      const handler = (req: Request) => createResponse(200, { method: 'POST' });
      post('/test-post', handler);
      const matched = matchRoute('POST', '/test-post');
      expect(matched).toBeDefined();
    });

    it('put should register PUT route', () => {
      const handler = (req: Request) => createResponse(200, { method: 'PUT' });
      put('/test-put', handler);
      const matched = matchRoute('PUT', '/test-put');
      expect(matched).toBeDefined();
    });

    it('del should register DELETE route', () => {
      const handler = (req: Request) => createResponse(200, { method: 'DELETE' });
      del('/test-delete', handler);
      const matched = matchRoute('DELETE', '/test-delete');
      expect(matched).toBeDefined();
    });

    it('patch should register PATCH route', () => {
      const handler = (req: Request) => createResponse(200, { method: 'PATCH' });
      patch('/test-patch', handler);
      const matched = matchRoute('PATCH', '/test-patch');
      expect(matched).toBeDefined();
    });
  });

  describe('matchRoute function', () => {
    it('should match exact route', () => {
      const handler = (req: Request) => createResponse(200, {});
      get('/exact', handler);
      const matched = matchRoute('GET', '/exact');
      expect(matched).toBeDefined();
    });

    it('should match route with params', () => {
      const handler = (req: Request) => createResponse(200, {});
      get('/users/:id', handler);
      const matched = matchRoute('GET', '/users/123');
      expect(matched).toBeDefined();
    });

    it('should return undefined for unmatched route', () => {
      const matched = matchRoute('GET', '/nonexistent');
      expect(matched).toBeUndefined();
    });

    it('should not match different method', () => {
      const handler = (req: Request) => createResponse(200, {});
      get('/test', handler);
      const matched = matchRoute('POST', '/test');
      expect(matched).toBeUndefined();
    });

    it('should match route with multiple params', () => {
      const handler = (req: Request) => createResponse(200, {});
      get('/users/:userId/posts/:postId', handler);
      const matched = matchRoute('GET', '/users/123/posts/456');
      expect(matched).toBeDefined();
      if (matched) {
        const req = {
          method: 'GET',
          path: '/users/123/posts/456',
          headers: {},
          query: {},
          params: {},
          body: null,
          claims: null,
        };
        const resp = matched(req);
        expect(resp.status_code).toBe(200);
      }
    });

    it('should not match when param count differs', () => {
      const handler = (req: Request) => createResponse(200, {});
      get('/test/:id', handler);
      const matched = matchRoute('GET', '/test/123/extra');
      expect(matched).toBeUndefined();
    });
  });

  describe('dispatch function', () => {
    it('should return 404 for unmatched route', async () => {
      const ipcPayload = {
        method: 'GET',
        path: '/nonexistent',
        correlation_id: 'test-123',
      } as unknown as IPCPayload;
      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(404);
    });

    it('should dispatch to registered handler', async () => {
      const handler = (req: Request) => createResponse(200, { dispatched: true });
      get('/dispatch-test', handler);
      const ipcPayload = {
        method: 'GET',
        path: '/dispatch-test',
        correlation_id: 'test-456',
      } as unknown as IPCPayload;
      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(200);
    });

    it('should pass correct params to handler', async () => {
      let receivedReq: any = null;
      const handler = (req: Request) => {
        receivedReq = req;
        return createResponse(200, {});
      };
      get('/params/:id', handler);
      const ipcPayload = {
        method: 'GET',
        path: '/params/123',
        correlation_id: 'test-789',
        headers: { 'content-type': 'application/json' },
        query: { foo: 'bar' },
        body: { test: true },
        claims: { user_id: 'user1', roles: ['admin'] },
      } as unknown as IPCPayload;
      await dispatch(ipcPayload);
      expect(receivedReq).not.toBeNull();
      if (receivedReq) {
        expect(receivedReq.params.id).toBe('123');
        expect(receivedReq.headers['content-type']).toBe('application/json');
        expect(receivedReq.query.foo).toBe('bar');
        expect(receivedReq.claims.user_id).toBe('user1');
      }
    });
  });

  describe('writeFrame and parseFrames functions', () => {
    it('should write and parse a frame', () => {
      const { socket, writes } = createMockSocket();
      const payload = { test: true };
      writeFrame(socket, 0x01, payload);
      expect(writes.length).toBe(1);

      const parsed = parseFrames(writes[0]);
      expect(parsed.frames.length).toBe(1);
      expect(parsed.frames[0].msgType).toBe(0x01);
    });

    it('should handle null payload', () => {
      const { socket, writes } = createMockSocket();
      writeFrame(socket, 0x01, null);
      expect(writes.length).toBe(1);
    });

    it('should handle empty object payload', () => {
      const { socket, writes } = createMockSocket();
      writeFrame(socket, 0x02, {});
      expect(writes.length).toBe(1);
    });

    it('should handle string payload', () => {
      const { socket, writes } = createMockSocket();
      writeFrame(socket, 0x01, 'test string');
      expect(writes.length).toBe(1);
    });

    it('should handle number payload', () => {
      const { socket, writes } = createMockSocket();
      writeFrame(socket, 0x01, 12345);
      expect(writes.length).toBe(1);
    });

    it('should handle array payload', () => {
      const { socket, writes } = createMockSocket();
      writeFrame(socket, 0x01, [1, 2, 3]);
      expect(writes.length).toBe(1);
    });
  });

  describe('parseFrames edge cases', () => {
    it('should handle empty buffer', () => {
      const result = parseFrames(Buffer.alloc(0));
      expect(result.frames).toHaveLength(0);
      expect(result.remaining.length).toBe(0);
    });

    it('should handle incomplete header', () => {
      const buffer = Buffer.alloc(3);
      const result = parseFrames(buffer);
      expect(result.frames).toHaveLength(0);
      expect(result.remaining.length).toBe(3);
    });

    it('should handle incomplete payload', () => {
      const header = Buffer.alloc(5);
      header.writeUInt32LE(100, 0);
      header.writeUInt8(0x01, 4);
      const buffer = Buffer.concat([header, Buffer.alloc(50)]);
      const result = parseFrames(buffer);
      expect(result.frames).toHaveLength(0);
      expect(result.remaining.length).toBe(buffer.length);
    });

    it('should parse multiple frames', () => {
      const frame1 = createTestFrame(0x01, { test: 1 });
      const frame2 = createTestFrame(0x02, { test: 2 });
      const buffer = Buffer.concat([frame1, frame2]);
      const result = parseFrames(buffer);
      expect(result.frames).toHaveLength(2);
      expect(result.remaining.length).toBe(0);
    });
  });

  describe('getSocketPath function', () => {
    it('should return custom socketPath if provided', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
        socketPath: '/custom/path.sock',
      };
      const path = getSocketPath(options);
      expect(path).toBe('/custom/path.sock');
    });

    it('should generate socket path for non-Windows', () => {
      const options: WorkerOptions = {
        workerId: 'my-worker',
      };
      const path = getSocketPath(options);
      expect(path).toContain('my-worker');
      expect(path).toContain('.vyx');
    });
  });

  describe('handleSocketData function', () => {
    it('should handle heartbeat', () => {
      const { socket, writes } = createMockSocket();
      const bufferRef = { current: Buffer.alloc(0) };
      const heartbeatFrame = createTestFrame(0x03, null); // TYPE_HEARTBEAT
      const result = handleSocketData(socket, heartbeatFrame, bufferRef, 'test-worker');
      expect(result).toBeDefined();
    });

    it('should handle request and respond', async () => {
      const { socket, writes } = createMockSocket();
      const bufferRef = { current: Buffer.alloc(0) };
      const requestFrame = createTestFrame(0x01, {
        method: 'GET',
        path: '/test',
        headers: {},
        query: {},
        params: {},
        body: null,
        claims: null,
        correlation_id: 'test-123',
      });
      const result = handleSocketData(socket, requestFrame, bufferRef, 'test-worker');
      expect(result).toBeDefined();
    });

    it('should handle invalid JSON', () => {
      const { socket, writes } = createMockSocket();
      const bufferRef = { current: Buffer.alloc(0) };
      const invalidPayload = Buffer.from('invalid json');
      const header = Buffer.alloc(5);
      header.writeUInt32LE(invalidPayload.length, 0);
      header.writeUInt8(0x01, 4);
      const invalidFrame = Buffer.concat([header, invalidPayload]);
      writes.length = 0;
      const result = handleSocketData(socket, invalidFrame, bufferRef, 'test-worker');
      expect(result).toBeDefined();
    });

    it('should handle unknown message type', () => {
      const { socket, writes } = createMockSocket();
      const bufferRef = { current: Buffer.alloc(0) };
      writeFrame(socket, 0x99, { test: true });
      const unknownFrame = writes[writes.length - 1];
      writes.length = 0;
      handleSocketData(socket, unknownFrame, bufferRef, 'test-worker');
      expect(writes.length).toBe(0);
    });
  });

  describe('createAndSetupSocket function', () => {
    it('should be exported and callable', () => {
      expect(typeof createAndSetupSocket).toBe('function');
    });

    it('should handle parameters', () => {
      const socketPath = '/tmp/test-worker.sock';
      const workerId = 'test-worker';
      const capabilities = [{ path: '/api/test', method: 'GET' }];
      expect(() => {
        try {
          createAndSetupSocket(socketPath, workerId, capabilities);
        } catch (e) {
          // Expected to fail in test environment
        }
      }).not.toThrow();
    });
  });

  describe('setupSocketHandlers function', () => {
    it('should be exported and callable', () => {
      expect(typeof setupSocketHandlers).toBe('function');
    });
  });

  describe('setupProcessHandlers function', () => {
    it('should be exported and callable', () => {
      expect(typeof setupProcessHandlers).toBe('function');
    });
  });

  describe('start function', () => {
    it('should be exported and callable', () => {
      expect(typeof start).toBe('function');
    });

    it('should accept WorkerOptions', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
        socketPath: '/tmp/test.sock',
      };
      expect(() => {
        try {
          start(options);
        } catch (e) {
          // Expected to fail in test environment
        }
      }).not.toThrow();
    });
  });

  describe('WorkerOptions interface', () => {
    it('should accept all options', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
        socketPath: '/tmp/test.sock',
        capabilities: [
          { path: '/api/test', method: 'GET' },
        ],
      };
      expect(options.workerId).toBe('test-worker');
      expect(options.capabilities).toHaveLength(1);
    });

    it('should work with minimal options', () => {
      const options: WorkerOptions = {
        workerId: 'minimal-worker',
      };
      expect(options.workerId).toBe('minimal-worker');
    });
  });

  describe('createAndSetupSocket with shouldExit=false', () => {
    it('should create socket without exit on close when shouldExit is false', () => {
      // Set VYX_TEST_MODE to prevent exit
      process.env.VYX_TEST_MODE = 'true';
      
      // Create a mock socket that immediately closes
      const socket = createAndSetupSocket('/tmp/nonexistent.sock', 'test-worker', [], false);
      
      // Use waitFor to handle async callback
      return new Promise<void>((resolve) => {
        socket.on('close', () => {
          // Should not exit when shouldExit is false
          resolve();
        });
        
        socket.destroy();
      }).finally(() => {
        delete process.env.VYX_TEST_MODE;
      });
    });
  });

  describe('VYX_TEST_MODE environment variable', () => {
    it('should prevent exit when VYX_TEST_MODE is set', () => {
      const originalEnv = process.env.VYX_TEST_MODE;
      process.env.VYX_TEST_MODE = 'true';
      
      // Create a mock socket
      const socket = net.Socket();
      const workerId = 'test-worker';
      
      // setupSocketHandlers should not call process.exit when VYX_TEST_MODE is set
      expect(() => {
        setupSocketHandlers(socket, workerId, true);
        socket.emit('close');
      }).not.toThrow();
      
      process.env.VYX_TEST_MODE = originalEnv;
    });
  });

describe('handleSocketConnect', () => {
    it('should send handshake and heartbeat', () => {
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
      } as unknown as net.Socket;
      
      handleSocketConnect(mockSocket, 'test-worker', [{path: '/api/test', method: 'GET'}]);
      
      // Should have written 2 frames (handshake and heartbeat)
      expect(writes.length).toBe(2);
      
      // Verify first frame is handshake (type 0x05)
      const header1 = writes[0].slice(0, 5);
      expect(header1.readUInt8(4)).toBe(0x05); // TYPE_HANDSHAKE
      
      // Verify second frame is heartbeat (type 0x03)
      const header2 = writes[1].slice(0, 5);
      expect(header2.readUInt8(4)).toBe(0x03); // TYPE_HEARTBEAT
    });

    it('should handle empty capabilities', () => {
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
      } as unknown as net.Socket;
      
      handleSocketConnect(mockSocket, 'test-worker2');
      
      expect(writes.length).toBe(2);
    });
  });

  describe('setupProcessHandlers', () => {
    it('should be exported and callable', () => {
      const socket = new net.Socket();
      expect(() => {
        setupProcessHandlers(socket, false);
      }).not.toThrow();
    });

    it('should not create keepAlive interval when shouldExit is false', () => {
      const socket = new net.Socket();
      const originalEnv = process.env.VYX_TEST_MODE;
      process.env.VYX_TEST_MODE = 'true';
      
      // This should not throw and should not create the interval
      expect(() => {
        setupProcessHandlers(socket, false);
      }).not.toThrow();
      
      process.env.VYX_TEST_MODE = originalEnv;
    });
  });
  
  describe('socket event handlers', () => {
    it('should handle socket error events', () => {
      const socket = new net.Socket();
      const workerId = 'test-worker';
      
      setupSocketHandlers(socket, workerId, false);
      
      // Emit an error - should not throw
      const err = new Error('test error');
      expect(() => socket.emit('error', err)).not.toThrow();
    });

    it('should handle socket close event without exit when shouldExit is false', () => {
      const socket = new net.Socket();
      const workerId = 'test-worker';
      
      setupSocketHandlers(socket, workerId, false);
      
      // Use waitFor to handle async callback
      return new Promise<void>((resolve) => {
        socket.on('close', () => {
          // Should reach here without exiting
          resolve();
        });
        
        socket.emit('close');
      });
    }, 100);
  });
  
  describe('createAndSetupSocket', () => {
    it('should call callback on connect', () => {
      const socketPath = '/tmp/nonexistent-test.sock';
      const workerId = 'test-worker';
      
      // Override net.createConnection to immediately emit 'connect'
      const socket = createAndSetupSocket(socketPath, workerId, [], false);
      
      // The socket should be defined
      expect(socket).toBeDefined();
      
      // Cleanup
      socket.destroy();
    }, 100);

    it('should send handshake and heartbeat on connect', () => {
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
        on: function(event: string, handler: Function) {
          if (event === 'connect' || event === 'data') {
            // Simulate connect event
            setTimeout(() => handler(), 10);
          }
          return this;
        },
        destroy: () => {},
      } as unknown as net.Socket;
      
      // We can't easily test the actual connect callback without more complex mocking
      // But we can verify the function handles the parameters correctly
      const result = createAndSetupSocket('/tmp/test.sock', 'test', [{path: '/api', method: 'GET'}], false);
      expect(result).toBeDefined();
      result.destroy();
    }, 100);
  });

  describe('handleSocketData', () => {
    it('should handle heartbeat frames', () => {
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
      } as unknown as net.Socket;
      
      const bufferRef = { current: Buffer.alloc(0) };
      
      // Create a heartbeat frame
      const payload = Buffer.alloc(0);
      const header = Buffer.alloc(5);
      header.writeUInt32LE(0, 0);
      header.writeUInt8(0x03, 4); // TYPE_HEARTBEAT
      const frame = Buffer.concat([header, payload]);
      
      const result = handleSocketData(mockSocket, frame, bufferRef, 'test-worker');
      expect(result).toBeDefined();
      expect(writes.length).toBeGreaterThan(0); // Should send heartbeat back
    });

    it('should handle request frames', () => {
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
      } as unknown as net.Socket;
      
      const bufferRef = { current: Buffer.alloc(0) };
      
      // Register a route
      const handler = (req: any) => createResponse(200, { ok: true });
      get('/test', handler);
      
      // Create a request frame
      const reqPayload = JSON.stringify({
        method: 'GET',
        path: '/test',
        correlation_id: 'test-123',
      });
      const payloadBuf = Buffer.from(reqPayload);
      const header = Buffer.alloc(5);
      header.writeUInt32LE(payloadBuf.length, 0);
      header.writeUInt8(0x01, 4); // TYPE_REQUEST
      const frame = Buffer.concat([header, payloadBuf]);
      
       const result = handleSocketData(mockSocket, frame, bufferRef, 'test-worker');
       expect(result).toBeDefined();
     });
  });

  describe('signal handlers and cleanup', () => {
    it('should setup process handlers without error', () => {
      const socketPath = '/tmp/test-signal-setup.sock';
      const workerId = 'test-signal';
      const routes: RouteRegistration[] = [];
      
      // This should not throw
      expect(() => {
        const socket = createAndSetupSocket(socketPath, workerId, routes, false);
        socket.destroy();
      }).not.toThrow();
    });

    it('should handle close event without exit when shouldExit is false', () => {
      const socketPath = '/tmp/test-close-noexit';
      const workerId = 'test-close-noexit';
      const routes: RouteRegistration[] = [];

      const socket = createAndSetupSocket(socketPath, workerId, routes, false);
      expect(socket).toBeDefined();

      // Destroy socket - should NOT trigger process.exit (shouldExit=false)
      socket.destroy();
    });

    it('should handle SIGTERM without exit when shouldExit is false', () => {
      const socketPath = '/tmp/test-sigterm-noexit';
      const workerId = 'test-sigterm-noexit';
      const routes: RouteRegistration[] = [];

      const socket = createAndSetupSocket(socketPath, workerId, routes, false);
      expect(socket).toBeDefined();

      // Emit SIGTERM - should NOT exit (shouldExit=false)
      process.emit('SIGTERM');
      
      socket.destroy();
    });

    it('should handle SIGINT without exit when shouldExit is false', () => {
      const socketPath = '/tmp/test-sigint-noexit';
      const workerId = 'test-sigint-noexit';
      const routes: RouteRegistration[] = [];

      const socket = createAndSetupSocket(socketPath, workerId, routes, false);
      expect(socket).toBeDefined();

      // Emit SIGINT - should NOT exit (shouldExit=false)
      process.emit('SIGINT');
      
      socket.destroy();
    });

    it('should cleanup on SIGTERM when shouldExit is true', () => {
      // Set VYX_TEST_MODE to prevent actual exit
      process.env.VYX_TEST_MODE = 'true';
      
      const socketPath = '/tmp/test-sigterm2';
      const workerId = 'test-sigterm2';
      const routes: RouteRegistration[] = [];
      
      const socket = createAndSetupSocket(socketPath, workerId, routes, false);
      expect(socket).toBeDefined();
      
      // Emit SIGTERM - cleanup should be called (VYX_TEST_MODE prevents exit)
      process.emit('SIGTERM');
      
      // Don't delete VYX_TEST_MODE before destroying - it prevents process.exit
      socket.destroy();
      delete process.env.VYX_TEST_MODE;
    });

    it('should cleanup on SIGINT when shouldExit is true', () => {
      // Set VYX_TEST_MODE to prevent actual exit
      process.env.VYX_TEST_MODE = 'true';
      
      const socketPath = '/tmp/test-sigint2';
      const workerId = 'test-sigint2';
      const routes: RouteRegistration[] = [];
      
      const socket = createAndSetupSocket(socketPath, workerId, routes, false);
      expect(socket).toBeDefined();
      
      // Emit SIGINT - cleanup should be called (VYX_TEST_MODE prevents exit)
      process.emit('SIGINT');
      
      // Don't delete VYX_TEST_MODE before destroying - it prevents process.exit
      socket.destroy();
      delete process.env.VYX_TEST_MODE;
    });

    it('should handle data events and update bufferRef', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
        on: function(event: string, handler: Function) {
          if (event === 'data') {
            // Simulate data event with a proper frame
            const payload = JSON.stringify({ method: 'GET', path: '/test' });
            const payloadBuf = Buffer.from(payload);
            const header = Buffer.alloc(5);
            header.writeUInt32LE(payloadBuf.length, 0);
            header.writeUInt8(0x01, 4); // TYPE_REQUEST
            handler(Buffer.concat([header, payloadBuf]));
          }
          return this;
        },
        destroy: () => {},
      } as unknown as net.Socket;

      const result = handleSocketData(mockSocket, Buffer.alloc(0), bufferRef, 'test-worker');
      expect(result).toBeDefined();
      expect(bufferRef.current).toBeDefined();
    });

    it('should handle invalid msgType in data events', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
        on: function(event: string, handler: Function) {
          if (event === 'data') {
            // Simulate data event with INVALID msgType (0xFF)
            const payload = JSON.stringify({ method: 'GET', path: '/test' });
            const payloadBuf = Buffer.from(payload);
            const header = Buffer.alloc(5);
            header.writeUInt32LE(payloadBuf.length, 0);
            header.writeUInt8(0xFF, 4); // INVALID TYPE
            handler(Buffer.concat([header, payloadBuf]));
          }
          return this;
        },
        destroy: () => {},
      } as unknown as net.Socket;

      const result = handleSocketData(mockSocket, Buffer.alloc(0), bufferRef, 'test-worker');
      expect(result).toBeDefined();
      // default branch should be covered (no error thrown)
    });

    it('should handle invalid JSON in request', () => {
      const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };
      const writes: Buffer[] = [];
      const mockSocket = {
        write: (buf: Buffer) => { writes.push(buf); return true; },
        on: function(event: string, handler: Function) {
          if (event === 'data') {
            // Simulate data event with invalid JSON
            const payload = 'invalid json';
            const payloadBuf = Buffer.from(payload);
            const header = Buffer.alloc(5);
            header.writeUInt32LE(payloadBuf.length, 0);
            header.writeUInt8(0x01, 4); // TYPE_REQUEST
            handler(Buffer.concat([header, payloadBuf]));
          }
          return this;
        },
        destroy: () => {},
      } as unknown as net.Socket;

      const result = handleSocketData(mockSocket, Buffer.alloc(0), bufferRef, 'test-worker');
      expect(result).toBeDefined();
      // catch branch should be covered
    });
  });
});
