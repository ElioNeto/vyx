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
});
