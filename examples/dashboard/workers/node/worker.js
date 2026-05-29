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
  *, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }

  :root {
    --bg: #0a0a1a;
    --bg-alt: #0d0d24;
    --surface: #13132b;
    --surface-hover: #1a1a35;
    --surface-active: #222245;
    --border: #1e1e3a;
    --border-light: #2a2a4a;
    --primary: #6366f1;
    --primary-hover: #818cf8;
    --primary-glow: rgba(99,102,241,0.25);
    --secondary: #06b6d4;
    --success: #22c55e;
    --danger: #ef4444;
    --warning: #eab308;
    --text: #f1f5f9;
    --text-secondary: #94a3b8;
    --text-muted: #64748b;
    --sidebar-w: 260px;
    --radius: 12px;
    --radius-sm: 8px;
    --radius-xs: 6px;
    --shadow: 0 1px 3px rgba(0,0,0,0.3), 0 1px 2px rgba(0,0,0,0.2);
    --shadow-lg: 0 10px 40px rgba(0,0,0,0.4);
    --shadow-glow: 0 0 20px var(--primary-glow);
    --font: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  }

  html { scroll-behavior: smooth; }

  body {
    font-family: var(--font);
    display: flex;
    min-height: 100vh;
    background: var(--bg);
    color: var(--text);
    line-height: 1.6;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  /* ── Scrollbar ── */
  ::-webkit-scrollbar { width: 6px; height: 6px; }
  ::-webkit-scrollbar-track { background: transparent; }
  ::-webkit-scrollbar-thumb { background: var(--border-light); border-radius: 3px; }
  ::-webkit-scrollbar-thumb:hover { background: var(--text-muted); }

  /* ── Sidebar ── */
  .sidebar {
    width: var(--sidebar-w);
    background: var(--bg-alt);
    padding: 0;
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    border-right: 1px solid var(--border);
    position: sticky;
    top: 0;
    height: 100vh;
  }

  .sidebar-logo {
    padding: 24px 24px 20px;
    font-size: 22px;
    font-weight: 800;
    letter-spacing: -0.5px;
    border-bottom: 1px solid var(--border);
    background: linear-gradient(135deg, var(--primary), var(--secondary));
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .sidebar-logo span { color: var(--primary); -webkit-text-fill-color: var(--primary); }

  .sidebar-nav { padding: 12px 0; flex: 1; display: flex; flex-direction: column; gap: 2px; }

  .sidebar-nav a {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 24px;
    color: var(--text-secondary);
    text-decoration: none;
    font-size: 14px;
    font-weight: 500;
    transition: all 0.2s ease;
    border-left: 3px solid transparent;
    margin: 0 12px;
    border-radius: var(--radius-sm);
  }

  .sidebar-nav a:hover {
    color: var(--text);
    background: var(--surface-hover);
  }

  .sidebar-nav a.active {
    color: var(--primary);
    background: rgba(99,102,241,0.1);
    border-left-color: var(--primary);
  }

  .sidebar-nav a .icon { width: 20px; text-align: center; font-size: 16px; }

  .sidebar-footer {
    padding: 16px 24px;
    border-top: 1px solid var(--border);
    font-size: 12px;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .sidebar-footer .dot {
    width: 8px; height: 8px; border-radius: 50%;
    background: var(--success);
    box-shadow: 0 0 8px rgba(34,197,94,0.5);
    flex-shrink: 0;
  }

  /* ── Main layout ── */
  .main { flex: 1; display: flex; flex-direction: column; min-width: 0; }

  .header {
    padding: 20px 32px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--border);
    background: rgba(10,10,26,0.8);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    position: sticky;
    top: 0;
    z-index: 10;
  }

  .header h1 {
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.3px;
    background: linear-gradient(135deg, var(--text), var(--text-secondary));
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .header-user {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 13px;
    color: var(--text-secondary);
  }

  .header-user .avatar {
    width: 36px; height: 36px;
    border-radius: 50%;
    background: linear-gradient(135deg, var(--primary), #8b5cf6);
    color: #fff;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 600;
    font-size: 14px;
    box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px rgba(99,102,241,0.2);
    transition: box-shadow 0.2s;
  }

  .header-user .avatar:hover { box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px rgba(99,102,241,0.4); }

  /* ── Content area ── */
  .content { padding: 32px; flex: 1; max-width: 1200px; }

  /* ── Cards ── */
  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 24px;
    margin-bottom: 24px;
    transition: border-color 0.2s, box-shadow 0.2s;
  }

  .card:hover {
    border-color: var(--border-light);
    box-shadow: var(--shadow);
  }

  .card h2 {
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
    margin-bottom: 16px;
    letter-spacing: -0.2px;
  }

  .card h3 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 8px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  /* ── Stats grid ── */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }

  .stat-card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 20px 24px;
    transition: all 0.2s ease;
    position: relative;
    overflow: hidden;
  }

  .stat-card::before {
    content: '';
    position: absolute;
    top: 0; left: 0;
    width: 100%; height: 3px;
    background: linear-gradient(90deg, var(--primary), var(--secondary));
    opacity: 0;
    transition: opacity 0.2s;
  }

  .stat-card:hover {
    border-color: var(--border-light);
    transform: translateY(-2px);
    box-shadow: var(--shadow-lg);
  }

  .stat-card:hover::before { opacity: 1; }

  .stat-card .label {
    font-size: 13px;
    color: var(--text-muted);
    margin-bottom: 8px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .stat-card .value {
    font-size: 28px;
    font-weight: 800;
    color: var(--text);
    letter-spacing: -1px;
    line-height: 1.2;
  }

  .stat-card .change {
    font-size: 13px;
    margin-top: 8px;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .stat-card .change.up { color: var(--success); }
  .stat-card .change.down { color: var(--danger); }
  .stat-card .change::before {
    font-size: 12px;
  }
  .stat-card .change.up::before { content: '▲ '; }
  .stat-card .change.down::before { content: '▼ '; }

  /* ── Chart ── */
  .chart-bar-container {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    height: 180px;
    padding: 8px 0;
  }

  .chart-bar {
    flex: 1;
    background: linear-gradient(to top, var(--primary), var(--secondary));
    border-radius: 4px 4px 0 0;
    min-width: 28px;
    position: relative;
    transition: all 0.3s ease;
    cursor: pointer;
  }

  .chart-bar:hover {
    opacity: 0.9;
    transform: scaleY(1.02);
    transform-origin: bottom;
    box-shadow: 0 0 20px var(--primary-glow);
  }

  .chart-bar .bar-value {
    position: absolute;
    top: -22px;
    left: 50%;
    transform: translateX(-50%);
    font-size: 11px;
    color: var(--text-secondary);
    white-space: nowrap;
    font-weight: 500;
  }

  .chart-labels {
    display: flex;
    gap: 8px;
    margin-top: 8px;
  }

  .chart-labels span {
    flex: 1;
    text-align: center;
    font-size: 12px;
    color: var(--text-muted);
    font-weight: 500;
  }

  /* ── Forms ── */
  .form-group { margin-bottom: 20px; }

  .form-group label {
    display: block;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 6px;
    letter-spacing: 0.2px;
  }

  .form-group input,
  .form-group select {
    width: 100%;
    padding: 10px 14px;
    border: 1px solid var(--border-light);
    border-radius: var(--radius-sm);
    font-size: 14px;
    font-family: var(--font);
    outline: none;
    transition: all 0.2s;
    background: var(--bg);
    color: var(--text);
  }

  .form-group input::placeholder {
    color: var(--text-muted);
  }

  .form-group input:focus,
  .form-group select:focus {
    border-color: var(--primary);
    box-shadow: 0 0 0 3px var(--primary-glow);
  }

  .form-group input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    background: var(--surface);
  }

  .form-group select {
    cursor: pointer;
    appearance: none;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' fill='%2394a3b8' viewBox='0 0 16 16'%3E%3Cpath d='M8 11L3 6h10z'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 12px center;
    padding-right: 36px;
  }

  /* ── Buttons ── */
  .btn {
    padding: 10px 24px;
    border: none;
    border-radius: var(--radius-sm);
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    font-family: var(--font);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    letter-spacing: 0.2px;
  }

  .btn-primary {
    background: linear-gradient(135deg, var(--primary), #8b5cf6);
    color: #fff;
    box-shadow: 0 4px 14px var(--primary-glow);
  }

  .btn-primary:hover {
    transform: translateY(-1px);
    box-shadow: 0 6px 20px var(--primary-glow);
  }

  .btn-primary:active { transform: translateY(0); }

  .btn-block { width: 100%; }

  /* ── Login page ── */
  .login-page {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg);
    position: relative;
    overflow: hidden;
  }

  .login-page::before {
    content: '';
    position: absolute;
    width: 600px; height: 600px;
    border-radius: 50%;
    background: radial-gradient(circle, rgba(99,102,241,0.15) 0%, transparent 70%);
    top: -200px; right: -200px;
    pointer-events: none;
  }

  .login-page::after {
    content: '';
    position: absolute;
    width: 500px; height: 500px;
    border-radius: 50%;
    background: radial-gradient(circle, rgba(6,182,212,0.1) 0%, transparent 70%);
    bottom: -150px; left: -150px;
    pointer-events: none;
  }

  .login-card {
    position: relative;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 40px;
    width: 420px;
    max-width: 90vw;
    box-shadow: var(--shadow-lg);
    backdrop-filter: blur(20px);
  }

  .login-card h1 {
    font-size: 26px;
    font-weight: 800;
    color: var(--text);
    margin-bottom: 8px;
    letter-spacing: -0.5px;
  }

  .login-card p {
    color: var(--text-muted);
    margin-bottom: 32px;
    font-size: 14px;
  }

  .error-msg {
    color: var(--danger);
    font-size: 13px;
    margin-top: 12px;
    padding: 10px 14px;
    background: rgba(239,68,68,0.1);
    border: 1px solid rgba(239,68,68,0.2);
    border-radius: var(--radius-xs);
  }

  .info-box {
    background: rgba(99,102,241,0.1);
    border: 1px solid rgba(99,102,241,0.2);
    border-radius: var(--radius-sm);
    padding: 14px 16px;
    margin-bottom: 24px;
    font-size: 13px;
    color: var(--primary-hover);
    line-height: 1.6;
  }

  .info-box strong { color: var(--primary); }

  /* ── Toggle row ── */
  .toggle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;
  }

  .toggle-row label:first-child { margin-bottom: 0; }

  /* ── Toggle switch ── */
  .switch {
    position: relative;
    display: inline-block;
    width: 48px;
    height: 26px;
    flex-shrink: 0;
  }

  .switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .slider {
    position: absolute;
    cursor: pointer;
    top: 0; left: 0; right: 0; bottom: 0;
    background: var(--border-light);
    transition: 0.3s;
    border-radius: 26px;
  }

  .slider::before {
    position: absolute;
    content: "";
    height: 20px;
    width: 20px;
    left: 3px;
    bottom: 3px;
    background: var(--text);
    transition: 0.3s;
    border-radius: 50%;
  }

  input:checked + .slider {
    background: linear-gradient(135deg, var(--primary), #8b5cf6);
  }

  input:checked + .slider::before {
    transform: translateX(22px);
    background: #fff;
  }

  /* ── Tables ── */
  table { width: 100%; border-collapse: collapse; }

  table thead tr {
    border-bottom: 2px solid var(--border);
  }

  table th {
    padding: 12px 16px;
    text-align: left;
    font-weight: 600;
    color: var(--text-muted);
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.8px;
  }

  table td {
    padding: 12px 16px;
    color: var(--text-secondary);
    font-size: 14px;
    border-bottom: 1px solid var(--border);
  }

  table tbody tr {
    transition: background 0.15s;
  }

  table tbody tr:hover { background: var(--surface-hover); }

  table tbody tr:last-child td { border-bottom: none; }

  /* ── Badges ── */
  .badge {
    display: inline-block;
    padding: 3px 12px;
    border-radius: 20px;
    font-size: 12px;
    font-weight: 600;
    letter-spacing: 0.2px;
  }

  .badge-admin {
    background: rgba(99,102,241,0.15);
    color: var(--primary);
    border: 1px solid rgba(99,102,241,0.25);
  }

  .badge-user {
    background: rgba(34,197,94,0.15);
    color: var(--success);
    border: 1px solid rgba(34,197,94,0.25);
  }

  /* ── Welcome alert ── */
  .welcome-box {
    background: linear-gradient(135deg, rgba(99,102,241,0.1), rgba(6,182,212,0.05));
    border: 1px solid rgba(99,102,241,0.2);
    border-radius: var(--radius);
    padding: 20px 24px;
    margin-bottom: 24px;
    font-size: 14px;
    color: var(--text-secondary);
    line-height: 1.6;
  }

  .welcome-box strong { color: var(--text); }

  /* ── Responsive ── */
  @media (max-width: 768px) {
    .sidebar { width: 60px; overflow: hidden; }
    .sidebar-logo { font-size: 0; padding: 20px 0; text-align: center; }
    .sidebar-logo::first-letter { font-size: 22px; }
    .sidebar-nav a { justify-content: center; padding: 12px 0; margin: 0 4px; }
    .sidebar-nav a .icon { margin: 0; }
    .sidebar-nav a span:not(.icon) { display: none; }
    .sidebar-footer { display: none; }
    .header { padding: 16px; }
    .content { padding: 16px; }
    .stats-grid { grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); }
    .header-user span { display: none; }
  }

  @media (max-width: 480px) {
    .stats-grid { grid-template-columns: 1fr; }
    .login-card { padding: 24px; }
  }
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
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" rel="stylesheet">
  <style>${STYLES}</style>
</head>
<body>
  <div class="sidebar">
    <div class="sidebar-logo">VYX<span>Dash</span></div>
    <nav class="sidebar-nav">
      ${navLinks}
    </nav>
    <div class="sidebar-footer">
      <span class="dot"></span>
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
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" rel="stylesheet">
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
    <div class="welcome-box">
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
      <div class="form-group toggle-row">
        <label>Notifications</label>
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
