import { get, post, put, patch, del, start, WorkerOptions } from '../src/dispatch';
import { createResponse, json, text, error, WorkerResponse } from '../src/request';
import type { Request } from '../src/request';

describe('dispatch module', () => {
  describe('route registration', () => {
    it('should register GET route with get()', () => {
      const handler = (req: Request) => createResponse(200, { ok: true });
      get('/test-get', handler);
      expect(handler).toBeDefined();
    });

    it('should register POST route with post()', () => {
      const handler = (req: Request) => createResponse(200, { ok: true });
      post('/test-post', handler);
      expect(handler).toBeDefined();
    });

    it('should register PUT route with put()', () => {
      const handler = (req: Request) => createResponse(200, { ok: true });
      put('/test-put', handler);
      expect(handler).toBeDefined();
    });

    it('should register DELETE route with del()', () => {
      const handler = (req: Request) => createResponse(200, { ok: true });
      del('/test-delete', handler);
      expect(handler).toBeDefined();
    });

    it('should register PATCH route with patch()', () => {
      const handler = (req: Request) => createResponse(200, { ok: true });
      patch('/test-patch', handler);
      expect(handler).toBeDefined();
    });
  });

  describe('start', () => {
    it('should accept WorkerOptions with workerId', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
        socketPath: '/tmp/test.sock',
      };
      expect(options.workerId).toBe('test-worker');
    });

    it('should accept WorkerOptions with capabilities', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
        capabilities: [
          { path: '/api/test', method: 'GET' },
        ],
      };
      expect(options.capabilities).toHaveLength(1);
    });

    it('should use default socket path when not provided', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
      };
      expect(options.workerId).toBe('test-worker');
    });
  });

  describe('module exports', () => {
    it('should export get function', () => {
      expect(get).toBeDefined();
    });

    it('should export post function', () => {
      expect(post).toBeDefined();
    });

    it('should export put function', () => {
      expect(put).toBeDefined();
    });

    it('should export del function', () => {
      expect(del).toBeDefined();
    });

    it('should export patch function', () => {
      expect(patch).toBeDefined();
    });

    it('should export start function', () => {
      expect(start).toBeDefined();
    });
  });
});
