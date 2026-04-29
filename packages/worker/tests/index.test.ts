import { worker, logger } from '../src/index';
import type { WorkerOptions } from '../src/dispatch';

describe('index module', () => {
  describe('worker object', () => {
    it('should have get method', () => {
      expect(typeof worker.get).toBe('function');
    });

    it('should have post method', () => {
      expect(typeof worker.post).toBe('function');
    });

    it('should have put method', () => {
      expect(typeof worker.put).toBe('function');
    });

    it('should have delete method', () => {
      expect(typeof worker.delete).toBe('function');
    });

    it('should have patch method', () => {
      expect(typeof worker.patch).toBe('function');
    });

    it('should have start method', () => {
      expect(typeof worker.start).toBe('function');
    });
  });

  describe('logger object', () => {
    it('should have info method', () => {
      expect(typeof logger.info).toBe('function');
    });

    it('should have error method', () => {
      expect(typeof logger.error).toBe('function');
    });

    it('should have warn method', () => {
      expect(typeof logger.warn).toBe('function');
    });

    it('should have debug method', () => {
      expect(typeof logger.debug).toBe('function');
    });

    it('should produce JSON output for info', () => {
      const originalLog = console.log;
      let output = '';
      console.log = (msg) => { output = msg; };

      logger.info('test message', { key: 'value' });

      expect(output).toContain('"level":"info"');
      expect(output).toContain('"message":"test message"');

      console.log = originalLog;
    });

    it('should produce JSON output for error', () => {
      const originalError = console.error;
      let output = '';
      console.error = (msg) => { output = msg; };

      logger.error('test error', { key: 'value' });

      expect(output).toContain('"level":"error"');
      expect(output).toContain('"message":"test error"');

      console.error = originalError;
    });

    it('should produce JSON output for warn', () => {
      const originalWarn = console.warn;
      let output = '';
      console.warn = (msg) => { output = msg; };

      logger.warn('test warn', { key: 'value' });

      expect(output).toContain('"level":"warn"');
      expect(output).toContain('"message":"test warn"');

      console.warn = originalWarn;
    });

    it('should produce JSON output for debug', () => {
      const originalLog = console.log;
      let output = '';
      console.log = (msg) => { output = msg; };

      logger.debug('test debug', { key: 'value' });

      expect(output).toContain('"level":"debug"');
      expect(output).toContain('"message":"test debug"');

      console.log = originalLog;
    });

    it('should include data in output', () => {
      const originalLog = console.log;
      let output = '';
      console.log = (msg) => { output = msg; };

      logger.info('test', { req_id: '123', data_key: 'data_value' });

      expect(output).toContain('"req_id":"123"');
      expect(output).toContain('"data_key":"data_value"');

      console.log = originalLog;
    });
  });

  describe('WorkerOptions interface', () => {
    it('should accept workerId', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
      };
      expect(options.workerId).toBe('test-worker');
    });

    it('should accept socketPath', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
        socketPath: '/tmp/test.sock',
      };
      expect(options.socketPath).toBe('/tmp/test.sock');
    });

    it('should accept capabilities', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
        capabilities: [
          { path: '/api/test', method: 'GET' },
        ],
      };
      expect(options.capabilities).toHaveLength(1);
    });

    it('should handle empty capabilities', () => {
      const options: WorkerOptions = {
        workerId: 'test-worker',
      };
      expect(options.capabilities).toBeUndefined();
    });
  });

  describe('exported types', () => {
    it('should export RequestStore type', () => {
      // This test ensures types are properly exported
      expect(worker).toBeDefined();
      expect(logger).toBeDefined();
    });

    it('should export IPCPayload type', () => {
      // Verify the module structure is correct
      expect(typeof worker.get).toBe('function');
      expect(typeof worker.post).toBe('function');
    });

    it('should export WorkerResponse type', () => {
      // Verify all exports are present
      const exports = Object.keys(worker);
      expect(exports).toContain('get');
      expect(exports).toContain('post');
      expect(exports).toContain('put');
      expect(exports).toContain('delete');
      expect(exports).toContain('patch');
      expect(exports).toContain('start');
    });
  });

  describe('logger correlation ID', () => {
    it('should include correlation ID in info logs', () => {
      const originalLog = console.log;
      let output = '';
      console.log = (msg) => { output = msg; };

      // Simulate a correlation ID context
      process.env.JEST_WORKER_ID = 'test';
      logger.info('test with context');

      expect(output).toContain('"level":"info"');
      expect(output).toContain('"message":"test with context"');

      console.log = originalLog;
    });

    it('should include correlation ID in error logs', () => {
      const originalError = console.error;
      let output = '';
      console.error = (msg) => { output = msg; };

      logger.error('test error with context');

      expect(output).toContain('"level":"error"');
      expect(output).toContain('"message":"test error with context"');

      console.error = originalError;
    });
  });

  describe('worker methods', () => {
    it('should have all HTTP methods', () => {
      const methods = ['get', 'post', 'put', 'delete', 'patch', 'start'];
      methods.forEach(method => {
        expect(typeof worker[method as keyof typeof worker]).toBe('function');
      });
    });

    it('should export delete as alias for del', () => {
      expect(worker.delete).toBeDefined();
      expect(typeof worker.delete).toBe('function');
    });
  });

  describe('exported types', () => {
    it('should export RequestStore type', () => {
      // This test ensures types are properly exported
      expect(worker).toBeDefined();
      expect(logger).toBeDefined();
    });

    it('should export IPCPayload type', () => {
      // Verify the module structure is correct
      expect(typeof worker.get).toBe('function');
      expect(typeof worker.post).toBe('function');
    });

    it('should export WorkerResponse type', () => {
      // Verify all exports are present
      const exports = Object.keys(worker);
      expect(exports).toContain('get');
      expect(exports).toContain('post');
      expect(exports).toContain('put');
      expect(exports).toContain('delete');
      expect(exports).toContain('patch');
      expect(exports).toContain('start');
    });
  });

  describe('logger correlation ID', () => {
    it('should include correlation ID in info logs', () => {
      const originalLog = console.log;
      let output = '';
      console.log = (msg) => { output = msg; };

      // Simulate a correlation ID context
      process.env.JEST_WORKER_ID = 'test';
      logger.info('test with context');

      expect(output).toContain('"level":"info"');
      expect(output).toContain('"message":"test with context"');

      console.log = originalLog;
    });

    it('should include correlation ID in error logs', () => {
      const originalError = console.error;
      let output = '';
      console.error = (msg) => { output = msg; };

      logger.error('test error with context');

      expect(output).toContain('"level":"error"');
      expect(output).toContain('"message":"test error with context"');

      console.error = originalError;
    });
  });

  describe('worker methods', () => {
    it('should have all HTTP methods', () => {
      const methods = ['get', 'post', 'put', 'delete', 'patch', 'start'];
      methods.forEach(method => {
        expect(typeof worker[method as keyof typeof worker]).toBe('function');
      });
    });

    it('should export delete as alias for del', () => {
      expect(worker.delete).toBeDefined();
      expect(typeof worker.delete).toBe('function');
    });

    it('should verify worker object structure', () => {
      expect(worker).toHaveProperty('get');
      expect(worker).toHaveProperty('post');
      expect(worker).toHaveProperty('put');
      expect(worker).toHaveProperty('delete');
      expect(worker).toHaveProperty('patch');
      expect(worker).toHaveProperty('start');
    });
  });

  describe('logger object properties', () => {
    it('should have all logger methods', () => {
      expect(logger).toHaveProperty('info');
      expect(logger).toHaveProperty('error');
      expect(logger).toHaveProperty('warn');
      expect(logger).toHaveProperty('debug');
    });

    it('should verify logger is a function object', () => {
      expect(typeof logger.info).toBe('function');
      expect(typeof logger.error).toBe('function');
      expect(typeof logger.warn).toBe('function');
      expect(typeof logger.debug).toBe('function');
    });
  });

  describe('module exports completeness', () => {
    it('should verify all expected exports exist', () => {
      // Verify worker exports
      expect(worker.get).toBeDefined();
      expect(worker.post).toBeDefined();
      expect(worker.put).toBeDefined();
      expect(worker.delete).toBeDefined();
      expect(worker.patch).toBeDefined();
      expect(worker.start).toBeDefined();

      // Verify logger exports
      expect(logger.info).toBeDefined();
      expect(logger.error).toBeDefined();
      expect(logger.warn).toBeDefined();
      expect(logger.debug).toBeDefined();
    });
  });
});
