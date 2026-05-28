/**
 * Node.js worker for the ecommerce vyx example.
 *
 * Handles order management and checkout flow.
 *
 * Wire protocol (matches core/infrastructure/ipc/framing/framing.go):
 *   [Length: 4 bytes LE][Type: 1 byte][Payload: N bytes]
 *
 * Annotated routes (parsed at build time by `vyx build`):
 *
 * @Route(GET /api/orders)
 * @Auth(roles: ["user", "admin"])
 *
 * @Route(POST /api/checkout)
 * @Auth(roles: ["user", "admin"])
 *
 * @Route(GET /api/orders/:id)
 * @Auth(roles: ["user", "admin"])
 */

'use strict';

const net = require('net');
const process = require('process');

// ─── IPC protocol constants ───────────────────────────────────────────────────
const TYPE_REQUEST   = 0x01;
const TYPE_RESPONSE  = 0x02;
const TYPE_HEARTBEAT = 0x03;
const TYPE_HANDSHAKE = 0x05;

// ─── In-memory store ──────────────────────────────────────────────────────────
// orders keyed by order ID
const orders = {};
// userOrders maps user ID -> array of order IDs
const userOrders = {};
let nextOrderId = 1000;

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

// @Route(GET /api/orders)
// @Auth(roles: ["user", "admin"])
function handleListOrders(req) {
  const sub = req.claims && req.claims.sub;
  const roles = req.claims && req.claims.roles || [];
  if (!sub) {
    return { status_code: 401, headers: { 'Content-Type': 'application/json' }, body: { error: 'unauthorized' } };
  }

  const isAdmin = Array.isArray(roles) && roles.includes('admin');
  let userOrderList;

  if (isAdmin) {
    // Admin can see all orders
    userOrderList = Object.values(orders);
  } else {
    // Regular user sees only their orders
    const userOrderIds = userOrders[sub] || [];
    userOrderList = userOrderIds.map(id => orders[id]).filter(Boolean);
  }

  return {
    status_code: 200,
    headers: { 'Content-Type': 'application/json' },
    body: { orders: userOrderList },
  };
}

// @Route(POST /api/checkout)
// @Auth(roles: ["user", "admin"])
function handleCheckout(req) {
  const sub = req.claims && req.claims.sub;
  if (!sub) {
    return { status_code: 401, headers: { 'Content-Type': 'application/json' }, body: { error: 'unauthorized' } };
  }

  const body = req.body || {};
  const orderId = String(nextOrderId++);

  const order = {
    id: orderId,
    user_id: sub,
    status: 'confirmed',
    shipping_address: body.shipping_address || '',
    payment_method: body.payment_method || '',
    notes: body.notes || '',
    created_at: new Date().toISOString(),
    items: body.items || [],
  };

  orders[orderId] = order;

  if (!userOrders[sub]) {
    userOrders[sub] = [];
  }
  userOrders[sub].push(orderId);

  return {
    status_code: 201,
    headers: { 'Content-Type': 'application/json' },
    body: { order, message: 'Checkout successful' },
  };
}

// @Route(GET /api/orders/:id)
// @Auth(roles: ["user", "admin"])
function handleGetOrder(req) {
  const id = req.params && req.params.id;
  const sub = req.claims && req.claims.sub;
  const roles = req.claims && req.claims.roles || [];

  if (!sub) {
    return { status_code: 401, headers: { 'Content-Type': 'application/json' }, body: { error: 'unauthorized' } };
  }

  const order = orders[id];
  if (!order) {
    return { status_code: 404, headers: { 'Content-Type': 'application/json' }, body: { error: `Order ${id} not found` } };
  }

  const isAdmin = Array.isArray(roles) && roles.includes('admin');
  // Users can only see their own orders, admins can see all
  if (!isAdmin && order.user_id !== sub) {
    return { status_code: 403, headers: { 'Content-Type': 'application/json' }, body: { error: 'forbidden' } };
  }

  return {
    status_code: 200,
    headers: { 'Content-Type': 'application/json' },
    body: order,
  };
}

function dispatch(req) {
  if (req.method === 'GET' && req.path === '/api/orders') return handleListOrders(req);
  if (req.method === 'POST' && req.path === '/api/checkout') return handleCheckout(req);
  if (req.method === 'GET' && req.path.startsWith('/api/orders/')) return handleGetOrder(req);
  return { status_code: 404, headers: { 'Content-Type': 'application/json' }, body: { error: 'route not found' } };
}

// ─── Connection ───────────────────────────────────────────────────────────────

const args = process.argv.slice(2);
let socketPath = process.platform === 'win32'
  ? '\\\\.\\pipe\\vyx-node:checkout'
  : '/tmp/vyx/node:checkout.sock';

for (let i = 0; i < args.length - 1; i++) {
  if (args[i] === '--vyx-socket') socketPath = args[i + 1];
}

console.log(`[node:checkout] connecting to ${socketPath}`);

const socket = net.createConnection(socketPath, () => {
  console.log('[node:checkout] connected to core');

  // Send handshake.
  const handshake = {
    type: 'handshake',
    worker_id: 'node:checkout',
    capabilities: [
      { path: '/api/orders',      method: 'GET' },
      { path: '/api/checkout',    method: 'POST' },
      { path: '/api/orders/:id',  method: 'GET' },
    ],
  };
  writeFrame(socket, TYPE_HANDSHAKE, handshake);
  console.log('[node:checkout] handshake sent');

  // Send an immediate heartbeat so the core marks this worker healthy
  // before the first 5-second monitor tick fires.
  writeFrame(socket, TYPE_HEARTBEAT, null);
  console.log('[node:checkout] initial heartbeat sent');
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
        console.log(`[node:checkout] ${req.method} ${req.path}`);
        const resp = dispatch(req);
        writeFrame(socket, TYPE_RESPONSE, resp);
        break;
      }

      default:
        break;
    }
  }
});

socket.on('error', (err) => console.error('[node:checkout] socket error:', err.message));
socket.on('close', () => { console.log('[node:checkout] disconnected'); process.exit(0); });

// Keep event loop alive
const keepAlive = setInterval(() => {}, 30_000);

process.on('SIGTERM', () => { clearInterval(keepAlive); socket.destroy(); process.exit(0); });
process.on('SIGINT',  () => { clearInterval(keepAlive); socket.destroy(); process.exit(0); });
