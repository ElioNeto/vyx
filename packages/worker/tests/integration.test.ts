import { runInRequestContextAsync, getCorrelationId } from '../src/context';
import type { IPCPayload } from '../src/request';

function mockDispatch(ipcPayload: IPCPayload): string {
  return getCorrelationId() ?? '';
}

describe('Integration: concurrent IPC requests', () => {
  it('should handle concurrent requests with isolated correlation IDs', async () => {
    const payloads: IPCPayload[] = [
      { method: 'GET', path: '/api/users', headers: {}, query: {}, params: {}, body: null, claims: null, correlation_id: 'req-001' },
      { method: 'GET', path: '/api/users', headers: {}, query: {}, params: {}, body: null, claims: null, correlation_id: 'req-002' },
      { method: 'GET', path: '/api/users', headers: {}, query: {}, params: {}, body: null, claims: null, correlation_id: 'req-003' },
    ];

    const results = await Promise.all(
      payloads.map((payload) =>
        runInRequestContextAsync(payload.correlation_id, async () => {
          await new Promise((resolve) => setTimeout(resolve, Math.random() * 5));
          return mockDispatch(payload);
        })
      )
    );

    expect(results).toEqual(['req-001', 'req-002', 'req-003']);
  });

  it('should echo correlation_id in response envelope', async () => {
    const payload: IPCPayload = {
      method: 'GET',
      path: '/api/users',
      headers: { 'X-Request-Id': 'test-echo-123' },
      query: {},
      params: {},
      body: null,
      claims: null,
      correlation_id: 'test-echo-123',
    };

    const result = runInRequestContextAsync(payload.correlation_id, async () => {
      const correlationId = getCorrelationId();
      return {
        status_code: 200,
        headers: { 'Content-Type': 'application/json' },
        body: { users: [] },
        correlation_id: correlationId,
      };
    });

    const response = await result;
    expect(response.correlation_id).toBe('test-echo-123');
  });

  it('should simulate realistic worker dispatch flow', async () => {
    const handler = async (payload: IPCPayload) => {
      const correlationId = getCorrelationId();

      await new Promise((resolve) => setTimeout(resolve, Math.random() * 10));

      return {
        status_code: 200,
        headers: { 'Content-Type': 'application/json' },
        body: { path: payload.path, correlationId },
        correlation_id: correlationId,
      };
    };

    const payloads: IPCPayload[] = [
      { method: 'GET', path: '/api/users', headers: {}, query: {}, params: {}, body: null, claims: null, correlation_id: 'flow-001' },
      { method: 'GET', path: '/api/products', headers: {}, query: {}, params: {}, body: null, claims: null, correlation_id: 'flow-002' },
      { method: 'POST', path: '/api/orders', headers: {}, query: {}, params: {}, body: {}, claims: null, correlation_id: 'flow-003' },
    ];

    const results = await Promise.all(
      payloads.map((payload) =>
        runInRequestContextAsync(payload.correlation_id, () => handler(payload))
      )
    );

    expect(results[0].body.correlationId).toBe('flow-001');
    expect(results[1].body.correlationId).toBe('flow-002');
    expect(results[2].body.correlationId).toBe('flow-003');

    expect(results[0].correlation_id).toBe('flow-001');
    expect(results[1].correlation_id).toBe('flow-002');
    expect(results[2].correlation_id).toBe('flow-003');
  });
});