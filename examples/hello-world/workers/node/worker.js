/**
 * Node.js worker for the hello-world vyx example.
 *
 * Connects to the vyx core via Unix Domain Socket (UDS) on Unix/macOS
 * or via Named Pipe on Windows, performs the handshake, and handles requests.
 *
 * Wire protocol (matches core/infrastructure/ipc/framing/framing.go):
 *   [Length: 4 bytes LE][Type: 1 byte][Payload: N bytes]
 *
 * Annotated routes (parsed at build time by `vyx build`):
 *
 * @Route(GET /api/products)
 * @Auth(roles: ["guest", "user"])
 *
 * @Route(GET /api/products/:id)
 * @Auth(roles: ["user"])
 */

'use strict';

const net = require('net');
const process = require('process');

// ─── IPC protocol constants ───────────────────────────────────────────────────
const TYPE_REQUEST   = 0x01;
const TYPE_RESPONSE  = 0x02;
const TYPE_HEARTBEAT = 0x03;
const TYPE_HANDSHAKE = 0x10;

// ─── Helpers ──────────────────────────────────────────────────────────────────

/**
 * Write a framed message.
 * Frame layout: [Length 4B LE][Type 1B][Payload NB]
 * Must match binary.LittleEndian used in framing.go.
 */
function writeFrame(socket, msgType, payload) {
  const payloadBuf = payload ? Buffer.from(JSON.stringify(payload)) : Buffer.alloc(0);
  const header = Buffer.alloc(5);
  header.writeUInt32LE(payloadBuf.length, 0); // little-endian — matches framing.go
  header.writeUInt8(msgType, 4);
  socket.write(Buffer.concat([header, payloadBuf]));
}

/**
 * Parse as many complete frames as possible from buffer.
 * Length field is 4 bytes LE (matches binary.LittleEndian in framing.go).
 */
function parseFrames(buffer) {
  const frames = [];
  let offset = 0;
  while (offset + 5 <= buffer.length) {
    const length = buffer.readUInt32LE(offset); // little-endian — matches framing.go
    const msgType = buffer.readUInt8(offset + 4);
    if (offset + 5 + length > buffer.length) break;
    const payload = buffer.slice(offset + 5, offset + 5 + length);
    frames.push({ msgType, payload });
    offset += 5 + length;
  }
  return { frames, remaining: buffer.slice(offset) };
}

// ─── Route handlers ───────────────────────────────────────────────────────────

// @Route(GET /api/products)
// @Auth(roles: ["guest", "user"])
function handleListProducts(req) {
  return {
    status_code: 200,
    headers: { 'Content-Type': 'application/json' },
    body: {
      products: [
        { id: '1', name: 'Widget Alpha', price: 9.99 },
        { id: '2', name: 'Widget Beta',  price: 19.99 },
        { id: '3', name: 'Widget Gamma', price: 4.99 },
      ],
    },
  };
}

// @Route(GET /api/products/:id)
// @Auth(roles: ["user"])
function handleGetProduct(req) {
  const id = req.params && req.params.id;
  const db = {
    '1': { id: '1', name: 'Widget Alpha', price: 9.99 },
    '2': { id: '2', name: 'Widget Beta',  price: 19.99 },
    '3': { id: '3', name: 'Widget Gamma', price: 4.99 },
  };
  const product = db[id];
  if (!product) {
    return {
      status_code: 404,
      headers: { 'Content-Type': 'application/json' },
      body: { error: `Product ${id} not found` },
    };
  }
  return {
    status_code: 200,
    headers: { 'Content-Type': 'application/json' },
    body: product,
  };
}

function dispatch(req) {
  if (req.method === 'GET' && req.path === '/api/products') return handleListProducts(req);
  if (req.method === 'GET' && req.path.startsWith('/api/products/')) return handleGetProduct(req);
  return { status_code: 404, headers: { 'Content-Type': 'application/json' }, body: { error: 'route not found' } };
}

// ─── Connection ───────────────────────────────────────────────────────────────

const args = process.argv.slice(2);
let socketPath = process.platform === 'win32'
  ? '\\\\.\\pipe\\vyx-node:api'
  : '/tmp/vyx/node:api.sock';

for (let i = 0; i < args.length - 1; i++) {
  if (args[i] === '--vyx-socket') socketPath = args[i + 1];
}

console.log(`[node:api] connecting to ${socketPath}`);

// net.createConnection supports Named Pipe paths on Windows natively.
const socket = net.createConnection(socketPath, () => {
  console.log('[node:api] connected to core');

  const handshake = {
    type: 'handshake',
    worker_id: 'node:api',
    capabilities: [
      { path: '/api/products',     method: 'GET' },
      { path: '/api/products/:id', method: 'GET' },
    ],
  };
  writeFrame(socket, TYPE_HANDSHAKE, handshake);
  console.log('[node:api] handshake sent');
});

let buffer = Buffer.alloc(0);

socket.on('data', (data) => {
  buffer = Buffer.concat([buffer, data]);
  const { frames, remaining } = parseFrames(buffer);
  buffer = remaining;

  for (const { msgType, payload } of frames) {
    switch (msgType) {
      case TYPE_HEARTBEAT:
        writeFrame(socket, TYPE_HEARTBEAT, null);
        break;

      case TYPE_REQUEST: {
        let req;
        try { req = JSON.parse(payload.toString()); } catch (e) { break; }
        console.log(`[node:api] ${req.method} ${req.path}`);
        const resp = dispatch(req);
        writeFrame(socket, TYPE_RESPONSE, resp);
        break;
      }

      default:
        // Unknown type — ignore silently (forward compat)
        break;
    }
  }
});

socket.on('error', (err) => console.error('[node:api] socket error:', err.message));
socket.on('close', () => { console.log('[node:api] disconnected'); process.exit(0); });

process.on('SIGTERM', () => { socket.destroy(); process.exit(0); });
process.on('SIGINT',  () => { socket.destroy(); process.exit(0); });
