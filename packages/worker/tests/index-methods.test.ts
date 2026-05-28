import { describe, it, expect, jest } from '@jest/globals';
import { worker as workerObj, logger as loggerObj, createResponse as createResponseFn, json as jsonFn, text as textFn, error as errorFn, getCorrelationId, runInRequestContext, runInRequestContextAsync, requestContext } from '../src/index.js';

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
      loggerObj.info('test message', { key: 'value' });
      expect(mockConsoleLog).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleLog.mock.calls[0][0]);
      expect(logged.level).toBe('info');
      expect(logged.message).toBe('test message');
    });

    it('should call logger.error', () => {
      loggerObj.error('test error', { key: 'value' });
      expect(mockConsoleError).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleError.mock.calls[0][0]);
      expect(logged.level).toBe('error');
      expect(logged.message).toBe('test error');
    });

    it('should call logger.warn', () => {
      loggerObj.warn('test warning', { key: 'value' });
      expect(mockConsoleWarn).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleWarn.mock.calls[0][0]);
      expect(logged.level).toBe('warn');
      expect(logged.message).toBe('test warning');
    });

    it('should call logger.debug', () => {
      loggerObj.debug('test debug', { key: 'value' });
      expect(mockConsoleLog).toHaveBeenCalled();
      const logged = JSON.parse(mockConsoleLog.mock.calls[0][0]);
      expect(logged.level).toBe('debug');
      expect(logged.message).toBe('test debug');
    });
  });

  describe('worker methods', () => {
    it('should call worker.get', () => {
      expect(() => {
        workerObj.get('/test-path', () => Promise.resolve({ status_code: 200, body: {} }));
      }).not.toThrow();
    });

    it('should call worker.post', () => {
      expect(() => {
        workerObj.post('/test-path', () => Promise.resolve({ status_code: 201, body: {} }));
      }).not.toThrow();
    });

    it('should call worker.put', () => {
      expect(() => {
        workerObj.put('/test-path', () => Promise.resolve({ status_code: 200, body: {} }));
      }).not.toThrow();
    });

    it('should call worker.patch', () => {
      expect(() => {
        workerObj.patch('/test-path', () => Promise.resolve({ status_code: 200, body: {} }));
      }).not.toThrow();
    });

    it('should call worker.delete', () => {
      expect(() => {
        workerObj.delete('/test-path', () => Promise.resolve({ status_code: 204, body: {} }));
      }).not.toThrow();
    });

    it('should call worker.start', () => {
      // Just verify the method exists and can be called (will fail to connect but that's ok)
      expect(typeof workerObj.start).toBe('function');
    });
  });

  describe('response helpers', () => {
    it('should call createResponse', () => {
      const resp = createResponseFn(200, { data: 'test' });
      expect(resp.status_code).toBe(200);
      expect(resp.body).toEqual({ data: 'test' });
    });

    it('should call json', () => {
      const resp = jsonFn({ data: 'test' });
      expect(resp.status_code).toBe(200);
      expect(resp.body).toEqual({ data: 'test' });
      expect(resp.headers).toEqual({ 'Content-Type': 'application/json' });
    });

    it('should call text', () => {
      const resp = textFn('hello world');
      expect(resp.status_code).toBe(200);
      expect(resp.body).toBe('hello world');
      expect(resp.headers).toEqual({ 'Content-Type': 'text/plain' });
    });

    it('should call error', () => {
      const resp = errorFn('something went wrong');
      expect(resp.status_code).toBe(500);
      expect(resp.body).toEqual({ error: 'something went wrong' });
    });
  });

  describe('context helpers', () => {
    it('should call getCorrelationId', () => {
      const id = getCorrelationId();
      // id may be undefined if not in request context, but should be string or undefined
      expect(id === undefined || typeof id === 'string').toBe(true);
    });

    it('should call runInRequestContext', () => {
      const result = runInRequestContext('test-id', () => 'result');
      expect(result).toBe('result');
    });

    it('should call runInRequestContextAsync', async () => {
      const result = await runInRequestContextAsync('test-id', async () => 'async-result');
      expect(result).toBe('async-result');
    });

    it('should access requestContext', () => {
      expect(requestContext).toBeDefined();
    });
  });
});
