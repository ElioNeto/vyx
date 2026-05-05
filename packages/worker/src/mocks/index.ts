import net from 'node:net';
import type { IPCPayload, WorkerResponse, Request, Claims } from '../request.js';
import type { WorkerOptions } from '../dispatch.js';

/**
 * Creates a mock net.Socket that records writes instead of sending them.
 * Useful for testing IPC frame serialization and protocol handling.
 *
 * @example
 * const { socket, writes } = createMockSocket();
 * writeFrame(socket, 0x01, { hello: 'world' });
 * expect(writes.length).toBe(1);
 */
export function createMockSocket(): { writes: Buffer[]; socket: net.Socket } {
  const writes: Buffer[] = [];
  const handlers: Record<string, Array<(...args: unknown[]) => void>> = {};

  return {
    writes,
    socket: {
      write: (buf: Buffer) => {
        writes.push(buf);
        return true;
      },
      on: (event: string, handler: (...args: unknown[]) => void) => {
        if (!handlers[event]) {
          handlers[event] = [];
        }
        handlers[event].push(handler);
        return {} as net.Socket;
      },
      emit: (event: string, ...args: unknown[]) => {
        const eventHandlers = handlers[event] ?? [];
        for (const handler of eventHandlers) {
          handler(...args);
        }
        return true;
      },
      destroy: () => {},
      destroySoon: () => {},
      end: () => {},
      setTimeout: () => ({}) as net.Socket,
      setEncoding: () => ({}) as net.Socket,
      address: () => ({ address: '0.0.0.0', port: 0, family: 'IPv4' }),
      unref: () => {},
      ref: () => {},
      connect: () => ({}) as net.Socket,
      setKeepAlive: () => ({}) as net.Socket,
      setNoDelay: () => ({}) as net.Socket,
      pause: () => ({}) as net.Socket,
      resume: () => ({}) as net.Socket,
      once: () => ({}) as net.Socket,
      addListener: () => ({}) as net.Socket,
      removeListener: () => ({}) as net.Socket,
      removeAllListeners: () => ({}) as net.Socket,
      listeners: () => [],
      rawListeners: () => [],
      listenerCount: () => 0,
      prependListener: () => ({}) as net.Socket,
      prependOnceListener: () => ({}) as net.Socket,
      eventNames: () => [],
      getMaxListeners: () => 0,
      setMaxListeners: () => ({}) as net.Socket,
      readonly: true,
      connecting: false,
      destroyed: false,
      localAddress: '',
      localPort: 0,
      remoteAddress: '',
      remotePort: 0,
      remoteFamily: '',
      pending: false,
      readyState: 'open' as const,
      bytesRead: 0,
      bytesWritten: 0,
    } as unknown as net.Socket,
  };
}

/**
 * Creates a binary frame buffer in the vyx IPC format.
 * Useful for testing parseFrames and handleSocketData.
 *
 * @param msgType - The message type byte (0x01=request, 0x02=response, 0x03=heartbeat, 0x05=handshake)
 * @param payload - The JSON-serializable payload
 * @returns A Buffer containing the header + payload
 */
export function createTestFrame(msgType: number, payload: unknown): Buffer {
  const payloadBuf = payload
    ? Buffer.from(JSON.stringify(payload))
    : Buffer.alloc(0);
  const header = Buffer.alloc(5);
  header.writeUInt32LE(payloadBuf.length, 0);
  header.writeUInt8(msgType, 4);
  return Buffer.concat([header, payloadBuf]);
}

/**
 * Creates a mock IPCPayload with sensible defaults.
 * Any provided overrides are merged into the defaults.
 *
 * @example
 * const payload = createMockIPCPayload({ path: '/api/users' });
 */
export function createMockIPCPayload(
  overrides?: Partial<IPCPayload>,
): IPCPayload {
  return {
    method: 'GET',
    path: '/',
    headers: {},
    query: {},
    params: {},
    body: null,
    claims: null,
    correlation_id: 'mock-correlation-id',
    ...overrides,
  };
}

/**
 * Creates mock WorkerOptions with sensible defaults.
 *
 * @example
 * const opts = createMockWorkerOptions({ workerId: 'my-worker' });
 */
export function createMockWorkerOptions(
  overrides?: Partial<WorkerOptions>,
): WorkerOptions {
  return {
    workerId: 'mock-worker',
    socketPath: '/tmp/mock-worker.sock',
    capabilities: [],
    ...overrides,
  };
}

/**
 * Creates a mock Request with sensible defaults.
 *
 * @example
 * const req = createMockRequest({ method: 'POST', body: { name: 'test' } });
 */
export function createMockRequest(overrides?: Partial<Request>): Request {
  return {
    method: 'GET',
    path: '/',
    headers: {},
    query: {},
    params: {},
    body: null,
    claims: null,
    ...overrides,
  };
}

/**
 * Creates a mock WorkerResponse with sensible defaults.
 * The correlation_id is auto-generated if not provided.
 */
export function createMockResponse(
  overrides?: Partial<WorkerResponse>,
): WorkerResponse {
  return {
    status_code: 200,
    body: { ok: true },
    correlation_id: 'mock-response-correlation',
    ...overrides,
  };
}

/**
 * Creates a mock Claims object.
 */
export function createMockClaims(overrides?: Partial<Claims>): Claims {
  return {
    user_id: 'mock-user',
    roles: ['user'],
    ...overrides,
  };
}
