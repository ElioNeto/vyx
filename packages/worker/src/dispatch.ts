import net from 'node:net';
import process from 'node:process';
import { runInRequestContextAsync } from './context.js';
import type { IPCPayload, WorkerResponse } from './request.js';
import os from 'node:os';
import path from 'node:path';
import fs from 'node:fs';

const TYPE_REQUEST = 0x01;
const TYPE_RESPONSE = 0x02;
const TYPE_HEARTBEAT = 0x03;
const TYPE_HANDSHAKE = 0x05;

function writeFrame(
  socket: net.Socket,
  msgType: number,
  payload: unknown
): void {
  const payloadBuf = payload
    ? Buffer.from(JSON.stringify(payload))
    : Buffer.alloc(0);
  const header = Buffer.alloc(5);
  header.writeUInt32LE(payloadBuf.length, 0);
  header.writeUInt8(msgType, 4);
  socket.write(Buffer.concat([header, payloadBuf] as [Buffer, Buffer]));
}

function parseFrames(buffer: Buffer): {
  frames: Array<{ msgType: number; payload: Buffer }>;
  remaining: Buffer;
} {
  const frames: Array<{ msgType: number; payload: Buffer }> = [];
  let offset = 0;
  while (offset + 5 <= buffer.length) {
    const length = buffer.readUInt32LE(offset);
    const msgType = buffer.readUInt8(offset + 4);
    if (offset + 5 + length > buffer.length) break;
    const payload = buffer.subarray(offset + 5, offset + 5 + length);
    frames.push({ msgType, payload });
    offset += 5 + length;
  }
  return {
    frames,
    remaining: buffer.subarray(offset),
  };
}

type RouteHandler = (req: {
  method: string;
  path: string;
  headers: Record<string, string>;
  query: Record<string, string>;
  params: Record<string, string>;
  body: unknown;
  claims: { user_id: string; roles: string[] } | null;
}) => WorkerResponse;

interface Route {
  path: string;
  method: string;
  handler: RouteHandler;
}

const routes: Route[] = [];

function addRoute(
  method: string,
  path: string,
  handler: RouteHandler
): void {
  routes.push({ path, method, handler });
}

function matchRoute(
  method: string,
  path: string
): RouteHandler | undefined {
  for (const route of routes) {
    if (route.method !== method) continue;

    const routeParts = route.path.split('/');
    const pathParts = path.split('/');

    if (routeParts.length !== pathParts.length) continue;

    let match = true;
    const params: Record<string, string> = {};

    for (let i = 0; i < routeParts.length; i++) {
      if (routeParts[i].startsWith(':')) {
        params[routeParts[i].slice(1)] = pathParts[i];
        continue;
      }
      if (routeParts[i] !== pathParts[i]) {
        match = false;
        break;
      }
    }

    if (match) {
      return (req: Parameters<RouteHandler>[0]) =>
        route.handler({ ...req, params });
    }
  }
  return undefined;
}

export function get(
  path: string,
  handler: RouteHandler
): void {
  addRoute('GET', path, handler);
}

export function post(
  path: string,
  handler: RouteHandler
): void {
  addRoute('POST', path, handler);
}

export function put(
  path: string,
  handler: RouteHandler
): void {
  addRoute('PUT', path, handler);
}

export function del(
  path: string,
  handler: RouteHandler
): void {
  addRoute('DELETE', path, handler);
}

export function patch(
  path: string,
  handler: RouteHandler
): void {
  addRoute('PATCH', path, handler);
}

async function dispatch(
  ipcPayload: IPCPayload
): Promise<WorkerResponse> {
  const handler = matchRoute(ipcPayload.method, ipcPayload.path);

  if (!handler) {
    return {
      status_code: 404,
      body: { error: 'route not found' },
      correlation_id: ipcPayload.correlation_id,
    };
  }

  const req = {
    method: ipcPayload.method,
    path: ipcPayload.path,
    headers: ipcPayload.headers,
    query: ipcPayload.query,
    params: ipcPayload.params,
    body: ipcPayload.body,
    claims: ipcPayload.claims,
  };

  return runInRequestContextAsync(ipcPayload.correlation_id, async () => handler(req));
}

export interface WorkerOptions {
  workerId: string;
  socketPath?: string;
  capabilities?: Array<{ path: string; method: string }>;
}

export function getSocketPath(options: WorkerOptions): string {
  // Check for --vyx-socket from command line (set by core when spawning worker)
  const vyxSocketIndex = process.argv.indexOf('--vyx-socket');
  if (vyxSocketIndex >= 0 && vyxSocketIndex < process.argv.length - 1) {
    return process.argv[vyxSocketIndex + 1];
  }

  return (
    options.socketPath ??
    (process.platform === 'win32'
      ? String.raw`\\.\pipe\vyx-${options.workerId}`
      : (() => {
        // Create socket in user-controlled directory with restricted permissions
        const safeDir = path.join(os.homedir(), '.vyx', 'sockets');
        fs.mkdirSync(safeDir, { recursive: true, mode: 0o700 });
        return path.join(safeDir, `vyx-${options.workerId}.sock`);
      })())
  );
}

export function start(options: WorkerOptions, shouldExit: boolean = true): void {
  const socketPath = getSocketPath(options);
  const socket = createAndSetupSocket(socketPath, options.workerId, options.capabilities, shouldExit);
  setupProcessHandlers(socket, shouldExit);
}

export function createAndSetupSocket(
  socketPath: string,
  workerId: string,
  capabilities?: Array<{ path: string; method: string }>,
  shouldExit: boolean = true
): net.Socket {
  const socket = net.createConnection(socketPath, () => {
    handleSocketConnect(socket, workerId, capabilities);
  });

  setupSocketHandlers(socket, workerId, shouldExit);
  return socket;
}

export function handleSocketConnect(
  socket: net.Socket,
  workerId: string,
  capabilities?: Array<{ path: string; method: string }>
): void {
  const handshake = {
    type: 'handshake',
    worker_id: workerId,
    capabilities: capabilities ?? [],
  };
  writeFrame(socket, TYPE_HANDSHAKE, handshake);

  writeFrame(socket, TYPE_HEARTBEAT, null);
}

export function setupSocketHandlers(
  socket: net.Socket,
  workerId: string,
  shouldExit: boolean = true
): void {
  const bufferRef: { current: Buffer } = { current: Buffer.alloc(0) };

  socket.on('data', (data: Buffer) => {
    const result = handleSocketData(socket, data, bufferRef, workerId);
    bufferRef.current = result.remaining;
  });

  socket.setKeepAlive(true);

  socket.on('error', (err) =>
    console.error(`[${workerId}] socket error:`, err.message)
  );
  socket.on('close', () => {
    // Don't exit in test mode or if VYX_TEST_MODE is set
    const isTestMode = !shouldExit || process.env.VYX_TEST_MODE || process.env.JEST_WORKER_ID;
    if (isTestMode) {
      return;
    }
    console.log(`[${workerId}] disconnected`);
    process.exit(0);
  });
}

export function setupProcessHandlers(socket: net.Socket, shouldExit: boolean = true): void {
  const shouldActuallyExit = shouldExit && !process.env.VYX_TEST_MODE;

  const cleanup = () => {
    socket.destroy();
  };

  process.on('SIGTERM', () => {
    cleanup();
    if (shouldActuallyExit && process.listenerCount('SIGTERM') <= 1) {
      process.exit(0);
    }
  });
  process.on('SIGINT', () => {
    cleanup();
    if (shouldActuallyExit && process.listenerCount('SIGINT') <= 1) {
      process.exit(0);
    }
  });
}

export interface HandleSocketDataResult {
  remaining: Buffer;
}

export function handleSocketData(
  socket: net.Socket,
  data: Buffer,
  bufferRef: { current: Buffer },
  workerId: string
): HandleSocketDataResult {
  bufferRef.current = Buffer.concat([bufferRef.current, data]);
  const result = parseFrames(bufferRef.current);

  for (const { msgType, payload } of result.frames) {
    switch (msgType) {
      case TYPE_HEARTBEAT:
        writeFrame(socket, TYPE_HEARTBEAT, null);
        break;

      case TYPE_REQUEST: {
        let req: IPCPayload;
        try {
          req = JSON.parse(payload.toString());
        } catch (e) {
          console.error(`[${workerId}] failed to parse request:`, e);
          break;
        }
        console.log(`[${workerId}] ${req.method} ${req.path}`);
        dispatch(req).then((resp) => {
          writeFrame(socket, TYPE_RESPONSE, resp);
        });
        break;
      }

      default:
        break;
    }
  }

  return { remaining: result.remaining };
}

// Exported for testing purposes
export { dispatch, matchRoute, writeFrame, parseFrames };
