/**
 * Node.js worker for the dashboard vyx example — SSR Pages.
 *
 * Connects to the vyx core via Unix Domain Socket (UDS) on Unix/macOS
 * or via Named Pipe on Windows, performs the handshake, and renders
 * dashboard pages as HTML with inline CSS.
 *
 * Wire protocol (matches core/infrastructure/ipc/framing/framing.go):
 *   [Length: 4 bytes LE][Type: 1 byte][Payload: N bytes]
 *
 * Annotated routes (parsed at build time by `vyx build`):
 *
 * @Page(/login)
 * @Auth(roles: ["guest"])
 *
 * @Page(/dashboard)
 * @Auth(roles: ["user", "admin"])
 *
 * @Page(/settings)
 * @Auth(roles: ["user", "admin"])
 */

'use strict';

const net = require('net');
const process = require('process');

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

// ─── CSS & HTML templates ─────────────────────────────────────────────────────

const STYLES = `
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; display: flex; min-height: 100vh; background: #f5f6fa; color: #333; }
.sidebar { width: 260px; background: #1a1a2e; color: #fff; padding: 24px 0; display: flex; flex-direction: column; flex-shrink: 0; }
.sidebar-logo { padding: 0 24px 24px; font-size: 20px; font-weight: 700; color: #fff; border-bottom: 1px solid rgba(255,255,255,0.08); }
.sidebar-logo span { color: #4361ee; }
.sidebar-nav { padding: 16px 0; flex: 1; }
.sidebar-nav a { display: flex; align-items: center; gap: 12px; padding: 12px 24px; color: rgba(255,255,255,0.6); text-decoration: none; font-size: 14px; transition: all 0.2s; }
.sidebar-nav a:hover, .sidebar-nav a.active { color: #fff; background: rgba(67,97,238,0.15); }
.sidebar-nav a .icon { width: 20px; text-align: center; }
.sidebar-footer { padding: 16px 24px; border-top: 1px solid rgba(255,255,255,0.08); font-size: 13px; color: rgba(255,255,255,0.4); }
.main { flex: 1; display: flex; flex-direction: column; }
.header { background: #fff; padding: 16px 32px; border-bottom: 1px solid #e8eaed; display: flex; justify-content: space-between; align-items: center; }
.header h1 { font-size: 20px; font-weight: 600; color: #1a1a2e; }
.header-user { display: flex; align-items: center; gap: 12px; font-size: 14px; color: #666; }
.header-user .avatar { width: 36px; height: 36px; border-radius: 50%; background: #4361ee; color: #fff; display: flex; align-items: center; justify-content: center; font-weight: 600; font-size: 14px; }
.content { padding: 32px; flex: 1; }
.card { background: #fff; border-radius: 12px; padding: 24px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); margin-bottom: 24px; }
.card h2 { font-size: 16px; font-weight: 600; color: #1a1a2e; margin-bottom: 16px; }
.card h3 { font-size: 14px; font-weight: 600; color: #666; margin-bottom: 8px; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-bottom: 24px; }
.stat-card { background: #fff; border-radius: 12px; padding: 20px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
.stat-card .label { font-size: 13px; color: #888; margin-bottom: 8px; }
.stat-card .value { font-size: 28px; font-weight: 700; color: #1a1a2e; }
.stat-card .change { font-size: 13px; margin-top: 4px; }
.stat-card .change.up { color: #2ecc71; }
.stat-card .change.down { color: #e74c3c; }
.chart-bar-container { display: flex; align-items: flex-end; gap: 8px; height: 160px; padding: 8px 0; }
.chart-bar { flex: 1; background: #4361ee; border-radius: 4px 4px 0 0; min-width: 24px; position: relative; transition: height 0.3s; }
.chart-bar:hover { opacity: 0.8; }
.chart-bar .bar-value { position: absolute; top: -20px; left: 50%; transform: translateX(-50%); font-size: 11px; color: #666; white-space: nowrap; }
.chart-labels { display: flex; gap: 8px; margin-top: 4px; }
.chart-labels span { flex: 1; text-align: center; font-size: 11px; color: #888; }
.form-group { margin-bottom: 20px; }
.form-group label { display: block; font-size: 14px; font-weight: 500; color: #333; margin-bottom: 6px; }
.form-group input, .form-group select { width: 100%; padding: 10px 12px; border: 1px solid #ddd; border-radius: 8px; font-size: 14px; outline: none; transition: border 0.2s; }
.form-group input:focus, .form-group select:focus { border-color: #4361ee; }
.btn { padding: 10px 24px; border: none; border-radius: 8px; font-size: 14px; font-weight: 500; cursor: pointer; transition: all 0.2s; }
.btn-primary { background: #4361ee; color: #fff; }
.btn-primary:hover { background: #3651d4; }
.btn-block { width: 100%; }
.login-page { min-height: 100vh; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%); }
.login-card { background: #fff; border-radius: 16px; padding: 40px; width: 400px; max-width: 90vw; box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
.login-card h1 { font-size: 24px; color: #1a1a2e; margin-bottom: 8px; }
.login-card p { color: #888; margin-bottom: 32px; font-size: 14px; }
.error-msg { color: #e74c3c; font-size: 13px; margin-top: 8px; }
.info-box { background: #eef2ff; border-radius: 8px; padding: 16px; margin-bottom: 24px; font-size: 13px; color: #4361ee; }
.switch { position: relative; display: inline-block; width: 48px; height: 26px; }
.switch input { opacity: 0; width: 0; height: 0; }
.slider { position: absolute; cursor: pointer; top: 0; left: 0; right: 0; bottom: 0; background: #ccc; transition: 0.3s; border-radius: 26px; }
.slider:before { position: absolute; content: ""; height: 20px; width: 20px; left: 3px; bottom: 3px; background: white; transition: 0.3s; border-radius: 50%; }
input:checked + .slider { background: #4361ee; }
input:checked + .slider:before { transform: translateX(22px); }
table { width: 100%; border-collapse: collapse; }
table th, table td { padding: 12px 16px; text-align: left; border-bottom: 1px solid #eee; font-size: 14px; }
table th { font-weight: 600; color: #666; font-size: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
table td { color: #333; }
.badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 12px; font-weight: 500; }
.badge-admin { background: #eef2ff; color: #4361ee; }
.badge-user { background: #f0fdf4; color: #2ecc71; }
`;

function renderLayout(title, content, user, activeTab) {
  const userName = (user && user.name) ? user.name : 'Guest';
  const userRole = (user && user.role) ? user.role : 'guest';
  const userInitial = userName.charAt(0).toUpperCase();
  const userEmail = (user && user.email) ? user.email : '';

  const navItems = [
    { href: '/dashboard', icon: '📊', label: 'Dashboard', tab: 'dashboard' },
    { href: '/settings', icon: '⚙️', label: 'Settings', tab: 'settings' },
  ];

  const navLinks = navItems.map(item => `
    <a href="${item.href}" class="${item.tab === activeTab ? 'active' : ''}">
      <span class="icon">${item.icon}</span> ${item.label}
    </a>
  `).join('');

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${title} — VYX Dashboard</title>
  <style>${STYLES}</style>
</head>
<body>
  <div class="sidebar">
    <div class="sidebar-logo">VYX<span>Dash</span></div>
    <nav class="sidebar-nav">
      ${navLinks}
    </nav>
    <div class="sidebar-footer">
      ${userName} &mdash; ${userRole}
    </div>
  </div>
  <div class="main">
    <div class="header">
      <h1>${title}</h1>
      <div class="header-user">
        <span>${userEmail}</span>
        <div class="avatar">${userInitial}</div>
      </div>
    </div>
    <div class="content">
      ${content}
    </div>
  </div>
</body>
</html>`;
}

// ─── Page renderers ───────────────────────────────────────────────────────────

// @Page(/login)
// @Auth(roles: ["guest"])
function renderLoginPage(req) {
  const error = (req.query && req.query.error) ? req.query.error : '';
  const errorHtml = error ? `<div class="error-msg">${escapeHtml(error)}</div>` : '';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Login — VYX Dashboard</title>
  <style>${STYLES}</style>
</head>
<body class="login-page">
  <div class="login-card">
    <h1>Welcome back</h1>
    <p>Sign in to your dashboard account</p>
    <div class="info-box">
      <strong>Demo credentials:</strong><br>
      Admin: admin / admin123<br>
      User: (any name) / password
    </div>
    <form method="POST" action="/api/auth/login" onsubmit="event.preventDefault(); login(this);">
      <div class="form-group">
        <label>Username</label>
        <input type="text" name="username" placeholder="admin" required minlength="3">
      </div>
      <div class="form-group">
        <label>Password</label>
        <input type="password" name="password" placeholder="••••••••" required minlength="6">
      </div>
      <button type="submit" class="btn btn-primary btn-block">Sign in</button>
      ${errorHtml}
    </form>
  </div>
  <script>
    async function login(form) {
      const data = { username: form.username.value, password: form.password.value };
      try {
        const res = await fetch('/api/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
        const json = await res.json();
        if (res.ok) {
          localStorage.setItem('vyx_token', json.token);
          localStorage.setItem('vyx_user', JSON.stringify(json.user));
          window.location.href = '/dashboard';
        } else {
          alert('Login failed: ' + (json.error || 'unknown error'));
        }
      } catch(e) {
        alert('Login error: ' + e.message);
      }
    }
  </script>
</body>
</html>`;
}

// @Page(/dashboard)
// @Auth(roles: ["user", "admin"])
function renderDashboardPage(req) {
  const userName = (req.claims && req.claims.name) ? req.claims.name : 'User';
  const userRole = (req.claims && req.claims.role) ? req.claims.role : 'user';

  // Mock analytics stats
  const stats = [
    { label: 'Total Users', value: '1,250', change: '+12.5%', up: true },
    { label: 'Active Sessions', value: '438', change: '+8.2%', up: true },
    { label: 'Page Views', value: '312.4K', change: '+23.1%', up: true },
    { label: 'Conversion Rate', value: '3.42%', change: '-0.8%', up: false },
  ];

  const statCards = stats.map(s => `
    <div class="stat-card">
      <div class="label">${s.label}</div>
      <div class="value">${s.value}</div>
      <div class="change ${s.up ? 'up' : 'down'}">${s.change}</div>
    </div>
  `).join('');

  // Mock chart data (monthly revenue)
  const chartData = [
    { label: 'Jan', value: 45 },
    { label: 'Feb', value: 52 },
    { label: 'Mar', value: 48 },
    { label: 'Apr', value: 61 },
    { label: 'May', value: 58 },
    { label: 'Jun', value: 65 },
  ];
  const maxVal = Math.max(...chartData.map(d => d.value));

  const bars = chartData.map(d => {
    const height = Math.round((d.value / maxVal) * 140);
    return `<div class="chart-bar" style="height:${height}px"><span class="bar-value">$${d.value}K</span></div>`;
  }).join('');

  const chartLabels = chartData.map(d => `<span>${d.label}</span>`).join('');

  // Mock users table
  const users = [
    { name: 'Admin User', email: 'admin@dashboard.local', role: 'admin' },
    { name: 'Regular User', email: 'user@dashboard.local', role: 'user' },
    { name: 'Alice Johnson', email: 'alice@example.com', role: 'user' },
  ];

  const userRows = users.map(u => `
    <tr>
      <td>${escapeHtml(u.name)}</td>
      <td>${escapeHtml(u.email)}</td>
      <td><span class="badge badge-${u.role}">${u.role}</span></td>
    </tr>
  `).join('');

  const content = `
    <div class="info-box">
      Welcome back, <strong>${escapeHtml(userName)}</strong>! Here is your dashboard overview.
    </div>

    <div class="stats-grid">${statCards}</div>

    <div class="card">
      <h2>Monthly Revenue</h2>
      <div class="chart-bar-container">${bars}</div>
      <div class="chart-labels">${chartLabels}</div>
    </div>

    <div class="card">
      <h2>Recent Users</h2>
      <table>
        <thead><tr><th>Name</th><th>Email</th><th>Role</th></tr></thead>
        <tbody>${userRows}</tbody>
      </table>
    </div>
  `;

  return renderLayout('Dashboard', content, req.claims, 'dashboard');
}

// @Page(/settings)
// @Auth(roles: ["user", "admin"])
function renderSettingsPage(req) {
  const userName = (req.claims && req.claims.name) ? req.claims.name : 'User';
  const userEmail = (req.claims && req.claims.email) ? req.claims.email : '';
  const userRole = (req.claims && req.claims.role) ? req.claims.role : 'user';
  const userId = (req.claims && req.claims.sub) ? req.claims.sub : '';

  const content = `
    <div class="card">
      <h2>Profile</h2>
      <div class="form-group">
        <label>Name</label>
        <input type="text" value="${escapeHtml(userName)}" disabled>
      </div>
      <div class="form-group">
        <label>Email</label>
        <input type="email" value="${escapeHtml(userEmail)}" disabled>
      </div>
      <div class="form-group">
        <label>Role</label>
        <input type="text" value="${escapeHtml(userRole)}" disabled>
      </div>
    </div>

    <div class="card">
      <h2>Preferences</h2>
      <div class="form-group">
        <label>Theme</label>
        <select id="theme">
          <option value="system">System</option>
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </div>
      <div class="form-group">
        <label>Language</label>
        <select id="language">
          <option value="en">English</option>
          <option value="pt">Português</option>
          <option value="es">Español</option>
        </select>
      </div>
      <div class="form-group">
        <label>Timezone</label>
        <select id="timezone">
          <option value="UTC">UTC</option>
          <option value="America/New_York">America/New_York</option>
          <option value="America/Sao_Paulo">America/Sao_Paulo</option>
          <option value="Europe/London">Europe/London</option>
        </select>
      </div>
      <div class="form-group" style="display:flex; align-items:center; gap:12px;">
        <label style="margin:0">Notifications</label>
        <label class="switch">
          <input type="checkbox" id="notifications" checked>
          <span class="slider"></span>
        </label>
      </div>
      <button class="btn btn-primary" onclick="saveSettings('${userId}')">Save Changes</button>
    </div>

    <script>
      async function saveSettings(userId) {
        const data = {
          theme: document.getElementById('theme').value,
          language: document.getElementById('language').value,
          timezone: document.getElementById('timezone').value,
          notifications_enabled: document.getElementById('notifications').checked,
        };
        const token = localStorage.getItem('vyx_token');
        try {
          const res = await fetch('/api/users/' + userId + '/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
            body: JSON.stringify(data),
          });
          if (res.ok) {
            alert('Settings saved!');
          } else {
            const json = await res.json();
            alert('Error: ' + (json.error || 'unknown'));
          }
        } catch(e) {
          alert('Error: ' + e.message);
        }
      }
    </script>
  `;

  return renderLayout('Settings', content, req.claims, 'settings');
}

// ─── Utility ──────────────────────────────────────────────────────────────────

function escapeHtml(str) {
  if (typeof str !== 'string') return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ─── Dispatch ─────────────────────────────────────────────────────────────────

function dispatch(req) {
  const path = req.path || '';
  const method = req.method || 'GET';
  const cleanPath = path.replace(/\/+$/, '') || '/';

  if (method === 'GET' && cleanPath === '/login') {
    return {
      status_code: 200,
      headers: { 'Content-Type': 'text/html; charset=utf-8' },
      body: renderLoginPage(req),
    };
  }

  if (method === 'GET' && cleanPath === '/dashboard') {
    return {
      status_code: 200,
      headers: { 'Content-Type': 'text/html; charset=utf-8' },
      body: renderDashboardPage(req),
    };
  }

  if (method === 'GET' && cleanPath === '/settings') {
    return {
      status_code: 200,
      headers: { 'Content-Type': 'text/html; charset=utf-8' },
      body: renderSettingsPage(req),
    };
  }

  return { status_code: 404, headers: { 'Content-Type': 'application/json' }, body: { error: 'route not found' } };
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
      { path: '/login',     method: 'GET' },
      { path: '/dashboard', method: 'GET' },
      { path: '/settings',  method: 'GET' },
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

// Keep the event loop alive so the socket does not get garbage collected.
const keepAlive = setInterval(() => {}, 30_000);

process.on('SIGTERM', () => { clearInterval(keepAlive); socket.destroy(); process.exit(0); });
process.on('SIGINT',  () => { clearInterval(keepAlive); socket.destroy(); process.exit(0); });
