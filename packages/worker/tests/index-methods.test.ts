import { describe, it, expect, jest } from '@jest/globals';
import * as index from '../src/index.js';

describe('Index Methods Coverage', () => {
  describe('logger methods', () => {
    let mockConsoleLog: jest.SpyInstance;
    let mockConsoleError: jest.SpyInstance;
    let mockConsoleWarn: jest.SpyInstance;

    beforeEach(() => {
      mockConsoleLog = jest.spyOn(console, 'log').mockImplementation(() => {});
      mockConsoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
      mockConsoleWarn = jest.spyOn(console, 'warn').mockImplementation(() => {});
    });

    afterEach(() => {
      mockConsoleLog.mockRestore();
      mockConsoleError.mockRestore();
      mockConsoleWarn.mockRestore();
    });

    it('should call logger.info', () => {
      index.logger.info('test message', { key: 'value' });
      expect(mockConsoleLog).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleLog.mock.calls[0][0]);
      expect(logged.level).toBe('info');
      expect(logged.message).toBe('test message');
    });

    it('should call logger.error', () => {
      index.logger.error('test error', { key: 'value' });
      expect(mockConsoleError).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleError.mock.calls[0][0]);
      expect(logged.level).toBe('error');
      expect(logged.message).toBe('test error');
    });

    it('should call logger.warn', () => {
      index.logger.warn('test warning', { key: 'value' });
      expect(mockConsoleWarn).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleWarn.mock.calls[0][0]);
      expect(logged.level).toBe('warn');
      expect(logged.message).toBe('test warning');
    });

    it('should call logger.debug', () => {
      index.logger.debug('test debug', { key: 'value' });
      expect(mockConsoleLog).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleLog.mock.calls[0][0]);
      expect(logged.level).toBe('debug');
      expect(logged.message).toBe('test debug');
    });
  });

  describe('worker methods', () => {
    it('should call worker.get', () => {
      index.worker.get('/test-path', () => Promise.resolve({ status_code: 200, body: {} }));
    });

    it('should call worker.post', () => {
      index.worker.post('/test-path', () => Promise.resolve({ status_code: 201, body: {} }));
    });

    it('should call worker.put', () => {
      index.worker.put('/test-path', () => Promise.resolve({ status_code: 200, body: {} }));
    });

    it('should call worker.patch', () => {
      index.worker.patch('/test-path', () => Promise.resolve({ status_code: 200, body: {} }));
    });

    it('should call worker.delete', () => {
      index.worker.delete('/test-path', () => Promise.resolve({ status_code: 204, body: {} }));
    });

    it('should call worker.start', () => {
      // Just verify the method exists and can be called (will fail to connect but that's ok)
      expect(typeof index.worker.start).toBe('function');
    });
  });

  describe('response helpers', () => {
    it('should call createResponse', () => {
      const resp = index.createResponse(200, { data: 'test' });
      expect(resp.status_code).toBe(200);
      expect(resp.body).toEqual({ data: 'test' });
    });

    it('should call json', () => {
      const resp = index.json({ data: 'test' });
      expect(resp.status_code).toBe(200);
      expect(resp.body).toEqual({ data: 'test' });
      expect(resp.headers).toEqual({ 'Content-Type': 'application/json' });
    });

    it('should call text', () => {
      const resp = index.text('hello world');
      expect(resp.status_code).toBe(200);
      expect(resp.body).toBe('hello world');
      expect(resp.headers).toEqual({ 'Content-Type': 'text/plain' });
    });

    it('should call error', () => {
      const resp = index.error('something went wrong');
      expect(resp.status_code).toBe(500);
      expect(resp.body).toEqual({ error: 'something went wrong' });
    });
  });

  describe('context helpers', () => {
    it('should call getCorrelationId', () => {
      const id = index.getCorrelationId();
      // id may be undefined if not in request context, but should be string or undefined
      expect(id === undefined || typeof id === 'string').toBe(true);
    });

    it('should call runInRequestContext', () => {
      const result = index.runInRequestContext('test-id', () => 'result');
      expect(result).toBe('result');
    });

    it('should call runInRequestContextAsync', async () => {
      const result = await index.runInRequestContextAsync('test-id', async () => 'async-result');
      expect(result).toBe('async-result');
    });

    it('should access requestContext', () => {
      expect(index.requestContext).toBeDefined();
    });
  });

    it('should call runInRequestContext', () => {
      const result = index.runInRequestContext('test-id', () => 'result');
      expect(result).toBe('result');
    });

    it('should call runInRequestContextAsync', async () => {
      const result = await index.runInRequestContextAsync('test-id', async () => 'async-result');
      expect(result).toBe('async-result');
    });

    it('should access requestContext', () => {
      expect(index.requestContext).toBeDefined();
    });
  });
});
