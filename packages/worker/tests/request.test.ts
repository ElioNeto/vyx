import {
  createResponse,
  json,
  text,
  error,
  WorkerResponse,
} from '../src/request';

describe('request module', () => {
  describe('createResponse', () => {
    it('should create basic response', () => {
      const resp = createResponse(200, { message: 'hello' });
      expect(resp.status_code).toBe(200);
      expect(resp.body).toEqual({ message: 'hello' });
      expect(resp.correlation_id).toBeDefined();
    });

    it('should create response with headers', () => {
      const resp = createResponse(201, { id: 123 }, { headers: { 'X-Custom': 'test' } });
      expect(resp.status_code).toBe(201);
      expect(resp.headers).toHaveProperty('X-Custom', 'test');
    });

    it('should handle null body', () => {
      const resp = createResponse(204, null);
      expect(resp.status_code).toBe(204);
      expect(resp.body).toBeNull();
    });
  });

  describe('json', () => {
    it('should create JSON response with default 200', () => {
      const resp = json({ key: 'value' });
      expect(resp.status_code).toBe(200);
      expect(resp.body).toEqual({ key: 'value' });
      expect(resp.headers).toHaveProperty('Content-Type', 'application/json');
    });

    it('should create JSON response with custom status', () => {
      const resp = json({ error: 'bad request' }, 400);
      expect(resp.status_code).toBe(400);
      expect(resp.body).toHaveProperty('error', 'bad request');
    });

    it('should handle array body', () => {
      const resp = json([1, 2, 3]);
      expect(resp.body).toEqual([1, 2, 3]);
    });
  });

  describe('text', () => {
    it('should create text response with default 200', () => {
      const resp = text('Hello World');
      expect(resp.status_code).toBe(200);
      expect(resp.body).toBe('Hello World');
      expect(resp.headers).toHaveProperty('Content-Type', 'text/plain');
    });

    it('should create text response with custom status', () => {
      const resp = text('Not Found', 404);
      expect(resp.status_code).toBe(404);
      expect(resp.body).toBe('Not Found');
    });
  });

  describe('error', () => {
    it('should create error response with default 500', () => {
      const resp = error('Something went wrong');
      expect(resp.status_code).toBe(500);
      expect(resp.body).toHaveProperty('error', 'Something went wrong');
      expect(resp.headers).toHaveProperty('Content-Type', 'application/json');
    });

    it('should create error response with custom status', () => {
      const resp = error('Unauthorized', 401);
      expect(resp.status_code).toBe(401);
      expect(resp.body).toHaveProperty('error', 'Unauthorized');
    });

    it('should handle empty message', () => {
      const resp = error('');
      expect(resp.status_code).toBe(500);
      expect(resp.body).toHaveProperty('error', '');
    });
  });

  describe('types', () => {
    it('should export correct types', () => {
      // TypeScript types are checked at compile time
      // Just verify the module loads
      expect(createResponse).toBeDefined();
      expect(json).toBeDefined();
      expect(text).toBeDefined();
      expect(error).toBeDefined();
    });
  });
});
