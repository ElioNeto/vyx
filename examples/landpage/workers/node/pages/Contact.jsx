/**
 * Contact Page - rendered by node:ssr worker
 *
 * @Page(/contact)
 * @Auth(roles: ["guest"])
 *
 * This page shows:
 * - Contact form (submits to POST /api/contact handled by go:api)
 * - Company contact information
 * - Office locations
 */

'use strict';

function renderContactPage() {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Contact - VYX Framework</title>
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
    .contact-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 48px; padding: 80px 0; align-items: start; }
    .contact-form h2, .contact-info h2 { font-size: 1.5rem; font-weight: 700; color: #0f172a; margin-bottom: 24px; }
    .form-group { margin-bottom: 20px; }
    .form-group label {
      display: block; font-size: 0.9rem; font-weight: 600; color: #374151;
      margin-bottom: 6px;
    }
    .form-group input, .form-group select, .form-group textarea {
      width: 100%; padding: 12px 14px; border: 1px solid #d1d5db;
      border-radius: 8px; font-size: 0.95rem; font-family: inherit;
      transition: border-color 0.2s; outline: none;
    }
    .form-group input:focus, .form-group select:focus, .form-group textarea:focus {
      border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37,99,235,0.1);
    }
    .form-group textarea { min-height: 140px; resize: vertical; }
    .form-group .hint { font-size: 0.8rem; color: #94a3b8; margin-top: 4px; }
    .btn-submit {
      background: #2563eb; color: #ffffff; border: none; padding: 14px 32px;
      border-radius: 8px; font-weight: 600; font-size: 1rem; cursor: pointer;
      transition: background 0.2s; width: 100%;
    }
    .btn-submit:hover { background: #1d4ed8; }
    .info-card {
      background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 12px;
      padding: 28px; margin-bottom: 20px;
    }
    .info-card h3 { font-size: 1.05rem; font-weight: 700; color: #0f172a; margin-bottom: 8px; }
    .info-card p { color: #64748b; font-size: 0.9rem; line-height: 1.7; }
    .info-card .icon { font-size: 1.3rem; margin-bottom: 8px; }
    .locations-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-top: 20px; }
    .location-card {
      background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 12px;
      padding: 20px;
    }
    .location-card h4 { font-size: 1rem; font-weight: 700; color: #0f172a; margin-bottom: 4px; }
    .location-card p { color: #64748b; font-size: 0.85rem; line-height: 1.6; }
    footer {
      background: #0f172a; color: #94a3b8; padding: 40px 24px; text-align: center;
    }
    footer a { color: #94a3b8; text-decoration: none; }
    footer a:hover { color: #ffffff; }
    .footer-links { display: flex; gap: 24px; justify-content: center; margin-bottom: 16px; flex-wrap: wrap; }
    .footer-links a { font-size: 0.9rem; }
    footer .copy { font-size: 0.85rem; }
    @media (max-width: 768px) {
      .contact-grid { grid-template-columns: 1fr; gap: 32px; }
      .locations-grid { grid-template-columns: 1fr; }
      .page-header h1 { font-size: 2rem; }
    }
  </style>
</head>
<body>
  <nav>
    <a href="/" class="nav-brand">VYX</a>
    <div class="nav-links">
      <a href="/">Home</a>
      <a href="/features">Features</a>
      <a href="/pricing">Pricing</a>
      <a href="/contact" class="active">Contact</a>
      <a href="/contact" class="btn-primary">Get Started</a>
    </div>
  </nav>

  <div class="page-header">
    <div class="container">
      <h1>Get in Touch</h1>
      <p>Have a question about VYX? Want to discuss enterprise plans? We'd love to hear from you.</p>
    </div>
  </div>

  <div class="container">
    <div class="contact-grid">
      <div class="contact-form">
        <h2>Send Us a Message</h2>
        <form action="/api/contact" method="POST">
          <div class="form-group">
            <label for="name">Full Name</label>
            <input type="text" id="name" name="name" placeholder="Your full name" required minlength="2">
          </div>
          <div class="form-group">
            <label for="email">Email Address</label>
            <input type="email" id="email" name="email" placeholder="you@example.com" required>
          </div>
          <div class="form-group">
            <label for="subject">Subject</label>
            <select id="subject" name="subject">
              <option value="general">General Inquiry</option>
              <option value="support">Technical Support</option>
              <option value="sales">Sales</option>
              <option value="partnership">Partnership</option>
            </select>
          </div>
          <div class="form-group">
            <label for="message">Message</label>
            <textarea id="message" name="message" placeholder="Tell us what you need..." required minlength="10"></textarea>
            <div class="hint">Minimum 10 characters. Maximum 2000 characters.</div>
          </div>
          <button type="submit" class="btn-submit">Send Message</button>
        </form>
      </div>
      <div class="contact-info">
        <h2>Contact Information</h2>
        <div class="info-card">
          <div class="icon">📧</div>
          <h3>Email</h3>
          <p>hello@vyx.dev<br>support@vyx.dev (technical)</p>
        </div>
        <div class="info-card">
          <div class="icon">💬</div>
          <h3>Community</h3>
          <p>Join our GitHub Discussions to ask questions, share ideas, and connect with other VYX developers.</p>
        </div>
        <div class="info-card">
          <div class="icon">🐙</div>
          <h3>GitHub</h3>
          <p>Star us on GitHub, report issues, and contribute to the open-source development of VYX.</p>
        </div>

        <h2 style="margin-top: 32px;">Our Locations</h2>
        <div class="locations-grid">
          <div class="location-card">
            <h4>San Francisco</h4>
            <p>548 Market St<br>San Francisco, CA 94104</p>
          </div>
          <div class="location-card">
            <h4>New York</h4>
            <p>175 Broadway<br>New York, NY 10013</p>
          </div>
          <div class="location-card">
            <h4>London</h4>
            <p>71 Queen Victoria St<br>London EC4V 4AY</p>
          </div>
          <div class="location-card">
            <h4>Tokyo</h4>
            <p>1-2-3 Shibuya<br>Tokyo 150-0002</p>
          </div>
        </div>
      </div>
    </div>
  </div>

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

module.exports = { renderContactPage };
