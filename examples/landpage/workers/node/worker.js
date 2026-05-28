/**
 * Node.js SSR worker for the landpage vyx example.
 *
 * Connects to the vyx core via Unix Domain Socket (UDS) on Unix/macOS
 * or via Named Pipe on Windows, performs the handshake, and renders pages.
 *
 * Wire protocol (matches core/infrastructure/ipc/framing/framing.go):
 *   [Length: 4 bytes LE][Type: 1 byte][Payload: N bytes]
 *
 * Annotated routes (parsed at build time by `vyx build`):
 *
 * @Page(/)
 * @Auth(roles: ["guest"])
 *
 * @Page(/features)
 * @Auth(roles: ["guest"])
 *
 * @Page(/pricing)
 * @Auth(roles: ["guest"])
 *
 * @Page(/contact)
 * @Auth(roles: ["guest"])
 */

'use strict';

const net = require('net');
const process = require('process');

const { renderHomePage } = require('./pages/Home.jsx');
const { renderFeaturesPage } = require('./pages/Features.jsx');
const { renderPricingPage } = require('./pages/Pricing.jsx');
const { renderContactPage } = require('./pages/Contact.jsx');

// ─── IPC protocol constants ───────────────────────────────────────────────────
// Must match core/domain/ipc/message.go
const TYPE_REQUEST   = 0x01;
const TYPE_RESPONSE  = 0x02;
const TYPE_HEARTBEAT = 0x03;
const TYPE_HANDSHAKE = 0x05;

// ─── Helpers ──────────────────────────────────────────────────────────────────

function writeFrame(socket, msgType, payload) {
  const payloadBuf = payload ? Buffer.from(JSON.stringify(payload)) : Buffer.alloc(0);
  const header = Buffer.alloc(5);
  header.writeUInt32LE(payloadBuf.length, 0);
  header.writeUInt8(msgType, 4);
  socket.write(Buffer.concat([header, payloadBuf]));
}

function parseFrames(buffer) {
  const frames = [];
  let offset = 0;
  while (offset + 5 <= buffer.length) {
    const length = buffer.readUInt32LE(offset);
    const msgType = buffer.readUInt8(offset + 4);
    if (offset + 5 + length > buffer.length) break;
    const payload = buffer.slice(offset + 5, offset + 5 + length);
    frames.push({ msgType, payload });
    offset += 5 + length;
  }
  return { frames, remaining: buffer.slice(offset) };
}

// ─── Route handlers ───────────────────────────────────────────────────────────

// @Page(/)
// @Auth(roles: ["guest"])
function handleHome(req) {
  return {
    status_code: 200,
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
    body: renderHomePage(),
  };
}

// @Page(/features)
// @Auth(roles: ["guest"])
function handleFeatures(req) {
  return {
    status_code: 200,
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
    body: renderFeaturesPage(),
  };
}

// @Page(/pricing)
// @Auth(roles: ["guest"])
function handlePricing(req) {
  return {
    status_code: 200,
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
    body: renderPricingPage(),
  };
}

// @Page(/contact)
// @Auth(roles: ["guest"])
function handleContact(req) {
  return {
    status_code: 200,
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
    body: renderContactPage(),
  };
}

function dispatch(req) {
  if (req.method === 'GET' && req.path === '/') return handleHome(req);
  if (req.method === 'GET' && req.path === '/features') return handleFeatures(req);
  if (req.method === 'GET' && req.path === '/pricing') return handlePricing(req);
  if (req.method === 'GET' && req.path === '/contact') return handleContact(req);
  return {
    status_code: 404,
    headers: { 'Content-Type': 'application/json' },
    body: { error: 'route not found' },
  };
}

// ─── Connection ───────────────────────────────────────────────────────────────

const args = process.argv.slice(2);
let socketPath = process.platform === 'win32'
  ? '\\\\.\\pipe\\vyx-node:ssr'
  : '/tmp/vyx/node:ssr.sock';

for (let i = 0; i < args.length - 1; i++) {
  if (args[i] === '--vyx-socket') socketPath = args[i + 1];
}

console.log(`[node:ssr] connecting to ${socketPath}`);

const socket = net.createConnection(socketPath, () => {
  console.log('[node:ssr] connected to core');

  // Send handshake.
  const handshake = {
    type: 'handshake',
    worker_id: 'node:ssr',
    capabilities: [
      { path: '/',           method: 'GET' },
      { path: '/features',   method: 'GET' },
      { path: '/pricing',    method: 'GET' },
      { path: '/contact',    method: 'GET' },
    ],
  };
  writeFrame(socket, TYPE_HANDSHAKE, handshake);
  console.log('[node:ssr] handshake sent');

  // Send an immediate heartbeat so the core marks this worker healthy
  // before the first 5-second monitor tick fires.
  writeFrame(socket, TYPE_HEARTBEAT, null);
  console.log('[node:ssr] initial heartbeat sent');
});

let buffer = Buffer.alloc(0);

socket.on('data', (data) => {
  buffer = Buffer.concat([buffer, data]);
  const { frames, remaining } = parseFrames(buffer);
  buffer = remaining;

  for (const { msgType, payload } of frames) {
    switch (msgType) {
      case TYPE_HEARTBEAT:
        // Echo the ping back to the core.
        writeFrame(socket, TYPE_HEARTBEAT, null);
        break;

      case TYPE_REQUEST: {
        let req;
        try { req = JSON.parse(payload.toString()); } catch (e) { break; }
        console.log(`[node:ssr] ${req.method} ${req.path}`);
        const resp = dispatch(req);
        writeFrame(socket, TYPE_RESPONSE, resp);
        break;
      }

      default:
        break;
    }
  }
});

socket.on('error', (err) => console.error('[node:ssr] socket error:', err.message));
socket.on('close', () => { console.log('[node:ssr] disconnected'); process.exit(0); });

// Mantém o event loop vivo para que o socket não seja coletado
const keepAlive = setInterval(() => {}, 30_000);

process.on('SIGTERM', () => { clearInterval(keepAlive); socket.destroy(); process.exit(0); });
process.on('SIGINT',  () => { clearInterval(keepAlive); socket.destroy(); process.exit(0); });
