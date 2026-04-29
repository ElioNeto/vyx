import { get, post, put, patch, del, dispatch, matchRoute, writeFrame, parseFrames, WorkerOptions } from '../src/dispatch';
import { createResponse, json, text, error, WorkerResponse } from '../src/request';
import type { Request, IPCPayload } from '../src/request';
import net from 'node:net';
import { Buffer } from 'node:buffer';

// Simple mock socket for testing writeFrame
function createMockSocket() {
  const writes: Buffer[] = [];
  return {
    writes,
    socket: {
      write: (buf: Buffer) => { writes.push(buf); return true; },
    } as unknown as net.Socket,
  };
}

describe('dispatch module - comprehensive coverage', () => {
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
    it('should match exact path without params', () => {
      const path = `/exact-${Date.now()}`;
      const handler = (req: Request) => createResponse(200, {});
      get(path, handler);

      const result = matchRoute('GET', path);
      expect(result).toBeDefined();
      expect(typeof result).toBe('function');
    });

    it('should match path with params via dispatch', async () => {
      const handler = (req: Request) => createResponse(200, { id: req.params.id });
      get('/users/:id', handler);

      const ipcPayload: IPCPayload = {
        method: 'GET',
        path: '/users/123',
        headers: {},
        query: {},
        params: {},
        body: null,
        claims: null,
        correlation_id: 'test-params',
      };

      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(200);
      expect(result.body).toHaveProperty('id', '123');
    });

    it('should not match different HTTP method', () => {
      const path = `/method-test-${Date.now()}`;
      const handler = (req: Request) => createResponse(200, {});
      get(path, handler);

      const result = matchRoute('POST', path);
      expect(result).toBeUndefined();
    });

    it('should return undefined for non-existent route', () => {
      const result = matchRoute('GET', `/non-existent-${Date.now()}`);
      expect(result).toBeUndefined();
    });
  });

  describe('dispatch function', () => {
    it('should return 404 for unmatched route', async () => {
      const ipcPayload: IPCPayload = {
        method: 'GET',
        path: `/non-existent-${Date.now()}`,
        headers: {},
        query: {},
        params: {},
        body: null,
        claims: null,
        correlation_id: 'test-404',
      };

      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(404);
      expect(result.body).toHaveProperty('error', 'route not found');
    });

    it('should dispatch to matched GET handler', async () => {
      const path = `/dispatch-get-${Date.now()}`;
      const handler = (req: Request) => createResponse(200, { dispatched: true });
      get(path, handler);

      const ipcPayload: IPCPayload = {
        method: 'GET',
        path,
        headers: {},
        query: {},
        params: {},
        body: null,
        claims: null,
        correlation_id: 'test-dispatch-get',
      };

      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(200);
      expect(result.body).toHaveProperty('dispatched', true);
    });

    it('should dispatch to matched POST handler with body', async () => {
      const path = `/dispatch-post-${Date.now()}`;
      const handler = (req: Request) => createResponse(201, { body: req.body });
      post(path, handler);

      const ipcPayload: IPCPayload = {
        method: 'POST',
        path,
        headers: {},
        query: {},
        params: {},
        body: { data: 'test' },
        claims: null,
        correlation_id: 'test-dispatch-post',
      };

      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(201);
      expect(result.body).toHaveProperty('body', { data: 'test' });
    });

    it('should pass headers to handler', async () => {
      const path = `/test-headers-${Date.now()}`;
      const handler = (req: Request) => createResponse(200, { header: req.headers['X-Test'] });
      get(path, handler);

      const ipcPayload: IPCPayload = {
        method: 'GET',
        path,
        headers: { 'X-Test': 'value' },
        query: {},
        params: {},
        body: null,
        claims: null,
        correlation_id: 'test-headers',
      };

      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(200);
      expect(result.body).toHaveProperty('header', 'value');
    });

    it('should pass claims to handler', async () => {
      const path = `/test-claims-${Date.now()}`;
      const handler = (req: Request) => createResponse(200, { user: req.claims?.user_id });
      get(path, handler);

      const ipcPayload: IPCPayload = {
        method: 'GET',
        path,
        headers: {},
        query: {},
        params: {},
        body: null,
        claims: { user_id: 'user1', roles: ['admin'] },
        correlation_id: 'test-claims',
      };

      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(200);
      expect(result.body).toHaveProperty('user', 'user1');
    });

    it('should handle WorkerResponse returned by handler', async () => {
      const path = `/worker-response-${Date.now()}`;
      const handler = (): WorkerResponse => ({
        status_code: 201,
        headers: { 'X-Custom': 'test-value' },
        body: { result: 'success' },
        correlation_id: 'test-worker-response',
      });
      post(path, handler);

      const ipcPayload: IPCPayload = {
        method: 'POST',
        path,
        headers: {},
        query: {},
        params: {},
        body: {},
        claims: null,
        correlation_id: 'test-worker-response',
      };

      const result = await dispatch(ipcPayload);
      expect(result.status_code).toBe(201);
      expect(result.headers).toHaveProperty('X-Custom', 'test-value');
    });
  });

  describe('writeFrame and parseFrames functions', () => {
    it('should write and parse a frame with payload', () => {
      const { socket, writes } = createMockSocket();

      const payload = { test: 'data', num: 123 };
      writeFrame(socket, 0x01, payload);

      expect(writes.length).toBe(1);
      const writtenBuffer = writes[0];

      const result = parseFrames(writtenBuffer);
      expect(result.frames).toHaveLength(1);
      expect(result.frames[0].msgType).toBe(0x01);
      expect(JSON.parse(result.frames[0].payload.toString())).toEqual(payload);
    });

    it('should write and parse a frame with null payload', () => {
      const { socket, writes } = createMockSocket();

      writeFrame(socket, 0x03, null);

      expect(writes.length).toBe(1);
      const writtenBuffer = writes[0];

      const result = parseFrames(writtenBuffer);
      expect(result.frames).toHaveLength(1);
      expect(result.frames[0].msgType).toBe(0x03);
      expect(result.frames[0].payload.length).toBe(0);
    });

    it('should handle multiple frames in buffer', () => {
      const { socket, writes } = createMockSocket();

      writeFrame(socket, 0x01, { first: 1 });
      writeFrame(socket, 0x02, { second: 2 });

      expect(writes.length).toBe(2);
      const combined = Buffer.concat([writes[0], writes[1]]);

      const result = parseFrames(combined);
      expect(result.frames).toHaveLength(2);
      expect(result.frames[0].msgType).toBe(0x01);
      expect(result.frames[1].msgType).toBe(0x02);
    });

    it('should handle partial frame', () => {
      const payload = { test: 'data' };
      const { socket, writes } = createMockSocket();

      writeFrame(socket, 0x01, payload);
      const writtenBuffer = writes[0];

      // Only pass partial buffer
      const partial = writtenBuffer.slice(0, 5); // Just the header
      const result = parseFrames(partial);

      expect(result.frames).toHaveLength(0);
      expect(result.remaining.length).toBe(5);
    });

    it('should handle empty buffer', () => {
      const result = parseFrames(Buffer.alloc(0));
      expect(result.frames).toHaveLength(0);
      expect(result.remaining.length).toBe(0);
    });

    it('should handle buffer with exact frame', () => {
      const { socket, writes } = createMockSocket();

      writeFrame(socket, 0x01, { exact: true });
      const writtenBuffer = writes[0];

      const result = parseFrames(writtenBuffer);
      expect(result.frames).toHaveLength(1);
      expect(result.remaining.length).toBe(0);
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
});
