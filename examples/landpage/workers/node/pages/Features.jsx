/**
 * Features Page - rendered by node:ssr worker
 *
 * @Page(/features)
 * @Auth(roles: ["guest"])
 *
 * This page shows:
 * - Detailed features list with icons and descriptions
 * - Architecture diagram section
 * - Technical specifications
 */

'use strict';

function renderFeaturesPage() {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Features - VYX Framework</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
      color: #1a202c; line-height: 1.6; background: #ffffff;
    }
    .container { max-width: 1200px; margin: 0 auto; padding: 0 24px; }
    nav {
      display: flex; align-items: center; justify-content: space-between;
      padding: 16px 24px; background: #ffffff; border-bottom: 1px solid #e2e8f0;
      position: sticky; top: 0; z-index: 100;
    }
    .nav-brand { font-size: 1.5rem; font-weight: 800; color: #2563eb; text-decoration: none; }
    .nav-links { display: flex; gap: 24px; align-items: center; }
    .nav-links a { text-decoration: none; color: #4a5568; font-weight: 500; font-size: 0.95rem; transition: color 0.2s; }
    .nav-links a:hover { color: #2563eb; }
    .nav-links .active { color: #2563eb; font-weight: 600; }
    .nav-links .btn-primary {
      background: #2563eb; color: #ffffff !important; padding: 8px 20px;
      border-radius: 6px; font-weight: 600; transition: background 0.2s;
    }
    .nav-links .btn-primary:hover { background: #1d4ed8; }
    .page-header {
      background: linear-gradient(180deg, #eff6ff 0%, #ffffff 100%);
      padding: 64px 24px 48px; text-align: center;
    }
    .page-header h1 { font-size: 2.8rem; font-weight: 800; color: #0f172a; margin-bottom: 12px; }
    .page-header p { color: #64748b; font-size: 1.15rem; max-width: 600px; margin: 0 auto; }
    section { padding: 80px 0; }
    .section-title { margin-bottom: 48px; }
    .section-title h2 { font-size: 2rem; font-weight: 700; color: #0f172a; margin-bottom: 12px; }
    .section-title p { color: #64748b; }
    .features-list { display: flex; flex-direction: column; gap: 32px; }
    .feature-row {
      display: flex; gap: 32px; align-items: flex-start;
      background: #f8fafc; border-radius: 12px; padding: 32px;
      border: 1px solid #e2e8f0; transition: box-shadow 0.3s;
    }
    .feature-row:hover { box-shadow: 0 4px 20px rgba(0,0,0,0.06); }
    .feature-row-icon {
      width: 56px; height: 56px; background: #eff6ff; border-radius: 14px;
      display: flex; align-items: center; justify-content: center;
      font-size: 1.6rem; flex-shrink: 0;
    }
    .feature-row-content h3 { font-size: 1.2rem; font-weight: 700; color: #0f172a; margin-bottom: 8px; }
    .feature-row-content p { color: #64748b; font-size: 0.95rem; line-height: 1.7; }
    .feature-tag {
      display: inline-block; background: #dbeafe; color: #1d4ed8;
      font-size: 0.75rem; font-weight: 600; padding: 3px 10px;
      border-radius: 20px; margin-top: 8px;
    }
    .tech-grid {
      display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 20px; margin-top: 16px;
    }
    .tech-card {
      background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px;
      padding: 24px; text-align: center;
    }
    .tech-card .tech-icon { font-size: 2rem; margin-bottom: 12px; }
    .tech-card h4 { font-size: 1rem; font-weight: 700; color: #0f172a; margin-bottom: 6px; }
    .tech-card p { color: #64748b; font-size: 0.85rem; }
    .specs-table { width: 100%; border-collapse: collapse; margin-top: 16px; }
    .specs-table td {
      padding: 14px 20px; border-bottom: 1px solid #e2e8f0; font-size: 0.95rem;
    }
    .specs-table td:first-child { font-weight: 600; color: #0f172a; width: 240px; }
    .specs-table td:last-child { color: #64748b; }
    .specs-table tr:last-child td { border-bottom: none; }
    footer {
      background: #0f172a; color: #94a3b8; padding: 40px 24px; text-align: center;
    }
    footer a { color: #94a3b8; text-decoration: none; }
    footer a:hover { color: #ffffff; }
    .footer-links { display: flex; gap: 24px; justify-content: center; margin-bottom: 16px; flex-wrap: wrap; }
    .footer-links a { font-size: 0.9rem; }
    footer .copy { font-size: 0.85rem; }
    @media (max-width: 768px) {
      .page-header h1 { font-size: 2rem; }
      .feature-row { flex-direction: column; }
      .specs-table td:first-child { width: auto; display: block; padding-bottom: 0; }
      .specs-table td:last-child { display: block; padding-top: 4px; }
    }
  </style>
</head>
<body>
  <nav>
    <a href="/" class="nav-brand">VYX</a>
    <div class="nav-links">
      <a href="/">Home</a>
      <a href="/features" class="active">Features</a>
      <a href="/pricing">Pricing</a>
      <a href="/contact">Contact</a>
      <a href="/contact" class="btn-primary">Get Started</a>
    </div>
  </nav>

  <div class="page-header">
    <div class="container">
      <h1>Everything You Need</h1>
      <p>A comprehensive set of tools and features to build, deploy, and scale your polyglot applications.</p>
    </div>
  </div>

  <section>
    <div class="container">
      <div class="section-title">
        <h2>Core Features</h2>
        <p>Every feature is designed to work seamlessly together, giving you a unified development experience.</p>
      </div>
      <div class="features-list">
        <div class="feature-row">
          <div class="feature-row-icon">⚡</div>
          <div class="feature-row-content">
            <h3>Polyglot Worker Architecture</h3>
            <p>VYX lets you run workers in Go, Node.js, and Python simultaneously. Each worker communicates with the Go core via a standardized IPC protocol over Unix Domain Sockets, meaning you can always use the best language for each task without sacrificing performance or developer experience.</p>
            <span class="feature-tag">Go</span>
            <span class="feature-tag">Node.js</span>
            <span class="feature-tag">Python</span>
          </div>
        </div>
        <div class="feature-row">
          <div class="feature-row-icon">📝</div>
          <div class="feature-row-content">
            <h3>Annotation-Based Routing</h3>
            <p>Declare routes directly in your source code using simple annotations. The VYX scanner parses Go, TypeScript, TSX, and Python files at build time, generating a route_map.json that the core uses to route incoming requests. No more maintaining separate routing configs.</p>
            <span class="feature-tag">@Route</span>
            <span class="feature-tag">@Page</span>
            <span class="feature-tag">@Auth</span>
            <span class="feature-tag">@Validate</span>
          </div>
        </div>
        <div class="feature-row">
          <div class="feature-row-icon">🔒</div>
          <div class="feature-row-content">
            <h3>Security & Validation</h3>
            <p>Built-in JWT authentication with role-based access control (RBAC), JSON Schema validation for request bodies, rate limiting per IP and per token, circuit breakers for fault tolerance, and configurable payload size limits. All layers are applied automatically based on your annotations.</p>
            <span class="feature-tag">JWT</span>
            <span class="feature-tag">RBAC</span>
            <span class="feature-tag">JSON Schema</span>
          </div>
        </div>
        <div class="feature-row">
          <div class="feature-row-icon">🔌</div>
          <div class="feature-row-content">
            <h3>Hot-Swappable Route Map</h3>
            <p>The RouteMap trie can be updated at runtime without restarting the core. When you modify routes or deploy new workers, VYX detects the changes and atomically swaps the routing table — lock-free reads via atomic pointers mean zero request drops during updates.</p>
            <span class="feature-tag">SIGHUP</span>
            <span class="feature-tag">Zero Downtime</span>
          </div>
        </div>
        <div class="feature-row">
          <div class="feature-row-icon">📊</div>
          <div class="feature-row-content">
            <h3>Observability & Monitoring</h3>
            <p>Integrated health checks, worker heartbeats, and a Bubble Tea TUI for real-time log tailing. VYX monitors worker liveness every 5 seconds and automatically restarts unhealthy workers after a configurable timeout.</p>
            <span class="feature-tag">Health Checks</span>
            <span class="feature-tag">TUI</span>
          </div>
        </div>
        <div class="feature-row">
          <div class="feature-row-icon">🚀</div>
          <div class="feature-row-content">
            <h3>High Performance IPC</h3>
            <p>Communication between the core and workers uses Unix Domain Sockets for low-latency, high-throughput data transfer. Payloads are serialized with MessagePack for small messages and Apache Arrow for large datasets, with automatic codec selection.</p>
            <span class="feature-tag">UDS</span>
            <span class="feature-tag">MsgPack</span>
            <span class="feature-tag">Apache Arrow</span>
          </div>
        </div>
      </div>
    </div>
  </section>

  <section style="background: #f8fafc;">
    <div class="container">
      <div class="section-title">
        <h2>Supported Technologies</h2>
        <p>VYX works with the languages and tools your team already knows.</p>
      </div>
      <div class="tech-grid">
        <div class="tech-card">
          <div class="tech-icon">🐹</div>
          <h4>Go</h4>
          <p>Core orchestration, API workers, high-throughput services</p>
        </div>
        <div class="tech-card">
          <div class="tech-icon">🟢</div>
          <h4>Node.js</h4>
          <p>SSR pages, REST APIs, real-time applications</p>
        </div>
        <div class="tech-card">
          <div class="tech-icon">🐍</div>
          <h4>Python</h4>
          <p>Data processing, ML inference, automation workers</p>
        </div>
        <div class="tech-card">
          <div class="tech-icon">⚛️</div>
          <h4>React / TSX</h4>
          <p>Server-side rendered pages with @Page annotations</p>
        </div>
      </div>
    </div>
  </section>

  <section>
    <div class="container">
      <div class="section-title">
        <h2>Technical Specifications</h2>
        <p>Under the hood details about how VYX works.</p>
      </div>
      <table class="specs-table">
        <tr><td>Core Language</td><td>Go (v1.25+)</td></tr>
        <tr><td>Worker Languages</td><td>Go, Node.js (≥20), Python (≥3.12)</td></tr>
        <tr><td>IPC Protocol</td><td>Binary framing: [4B LE length][1B type][payload]</td></tr>
        <tr><td>Transport</td><td>Unix Domain Sockets (Linux/macOS), Named Pipes (Windows)</td></tr>
        <tr><td>Serialization</td><td>MessagePack (small payloads), Apache Arrow (large datasets)</td></tr>
        <tr><td>Authentication</td><td>JWT (HS256/RS256) with RBAC</td></tr>
        <tr><td>Validation</td><td>JSON Schema (draft-07)</td></tr>
        <tr><td>Route Matching</td><td>Trie-based (O(k) with k = path segments)</td></tr>
        <tr><td>Fault Tolerance</td><td>Circuit Breaker (Closed → Open → Half-Open), Health Checks</td></tr>
        <tr><td>Rate Limiting</td><td>Per-IP and per-token sliding window</td></tr>
      </table>
    </div>
  </section>

  <footer>
    <div class="footer-links">
      <a href="/">Home</a>
      <a href="/features">Features</a>
      <a href="/pricing">Pricing</a>
      <a href="/contact">Contact</a>
    </div>
    <div class="copy">&copy; 2026 VYX Framework. All rights reserved.</div>
  </footer>
</body>
</html>`;
}

module.exports = { renderFeaturesPage };
