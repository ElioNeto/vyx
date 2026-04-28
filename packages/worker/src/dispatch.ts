import net from 'node:net';
import process from 'node:process';
import { runInRequestContextAsync } from './context.js';
import type { IPCPayload, WorkerResponse } from './request.js';

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
    const payload = buffer.slice(offset + 5, offset + 5 + length);
    frames.push({ msgType, payload });
    offset += 5 + length;
  }
  return {
    frames,
    remaining: buffer.slice(offset),
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

export function start(options: WorkerOptions): void {
  const socketPath =
    options.socketPath ??
    (process.platform === 'win32'
      ? String.raw`\\.\pipe\vyx-${options.workerId}`
      : (() => {
          // Create socket in user-controlled directory with restricted permissions
          const os = require('node:os') as typeof import('node:os');
          const path = require('node:path') as typeof import('node:path');
          const fs = require('node:fs') as typeof import('node:fs');

          const safeDir = path.join(os.homedir(), '.vyx', 'sockets');
          fs.mkdirSync(safeDir, { recursive: true, mode: 0o700 });
          return path.join(safeDir, `vyx-${options.workerId}.sock`);
        })());

  console.log(`[${options.workerId}] connecting to ${socketPath}`);

  const socket = net.createConnection(socketPath, () => {
    console.log(`[${options.workerId}] connected to core`);

    const handshake = {
      type: 'handshake',
      worker_id: options.workerId,
      capabilities: options.capabilities ?? [],
    };
    writeFrame(socket, TYPE_HANDSHAKE, handshake);
    console.log(`[${options.workerId}] handshake sent`);

    writeFrame(socket, TYPE_HEARTBEAT, null);
    console.log(`[${options.workerId}] initial heartbeat sent`);
  });

  let buffer: any = Buffer.alloc(0);

  socket.on('data', (data: any) => {
    buffer = Buffer.concat([buffer, data]);
    const result = parseFrames(buffer);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    buffer = result.remaining as any;

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
            console.error(`[${options.workerId}] failed to parse request:`, e);
            break;
          }
          console.log(`[${options.workerId}] ${req.method} ${req.path}`);
          dispatch(req).then((resp) => {
            writeFrame(socket, TYPE_RESPONSE, resp);
          });
          break;
        }

        default:
          break;
      }
    }
  });

  socket.on('error', (err) =>
    console.error(`[${options.workerId}] socket error:`, err.message)
  );
  socket.on('close', () => {
    console.log(`[${options.workerId}] disconnected`);
    process.exit(0);
  });

  const keepAlive = setInterval(() => {}, 30_000);

  process.on('SIGTERM', () => {
    clearInterval(keepAlive);
    socket.destroy();
    process.exit(0);
  });
  process.on('SIGINT', () => {
    clearInterval(keepAlive);
    socket.destroy();
    process.exit(0);
  });
}