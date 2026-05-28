/**
 * Pricing Page - rendered by node:ssr worker
 *
 * @Page(/pricing)
 * @Auth(roles: ["guest"])
 *
 * This page shows:
 * - Three pricing tiers (Starter, Professional, Enterprise)
 * - Feature comparison per tier
 * - FAQ section
 */

'use strict';

function renderPricingPage() {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Pricing - VYX Framework</title>
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
    .pricing-grid {
      display: grid; grid-template-columns: repeat(3, 1fr);
      gap: 24px; margin-top: 16px; align-items: start;
    }
    .pricing-card {
      background: #ffffff; border: 1px solid #e2e8f0; border-radius: 16px;
      padding: 40px 32px; transition: box-shadow 0.3s, transform 0.3s;
      position: relative;
    }
    .pricing-card:hover { box-shadow: 0 12px 40px rgba(0,0,0,0.08); transform: translateY(-4px); }
    .pricing-card.featured {
      border-color: #2563eb; border-width: 2px;
      transform: scale(1.02);
    }
    .pricing-card.featured:hover { transform: scale(1.02) translateY(-4px); }
    .pricing-badge {
      position: absolute; top: -12px; left: 50%; transform: translateX(-50%);
      background: #2563eb; color: #ffffff; font-size: 0.8rem; font-weight: 700;
      padding: 4px 16px; border-radius: 20px; text-transform: uppercase; letter-spacing: 0.5px;
    }
    .pricing-card .plan-name { font-size: 1.1rem; font-weight: 600; color: #64748b; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 16px; }
    .pricing-card .price { font-size: 3rem; font-weight: 800; color: #0f172a; margin-bottom: 4px; }
    .pricing-card .price span { font-size: 1.1rem; font-weight: 400; color: #94a3b8; }
    .pricing-card .description { color: #64748b; font-size: 0.9rem; margin-bottom: 24px; }
    .pricing-card .features { list-style: none; margin-bottom: 32px; }
    .pricing-card .features li {
      padding: 10px 0; border-bottom: 1px solid #f1f5f9;
      color: #475569; font-size: 0.9rem; display: flex; align-items: center; gap: 8px;
    }
    .pricing-card .features li::before { content: "✓"; color: #2563eb; font-weight: 700; }
    .pricing-card .features li.disabled { color: #cbd5e1; }
    .pricing-card .features li.disabled::before { content: "—"; color: #cbd5e1; }
    .pricing-card .features li:last-child { border-bottom: none; }
    .pricing-card .btn {
      display: block; width: 100%; text-align: center;
      padding: 14px; border-radius: 8px; font-weight: 600; font-size: 1rem;
      text-decoration: none; transition: background 0.2s;
    }
    .pricing-card .btn-primary { background: #2563eb; color: #ffffff; }
    .pricing-card .btn-primary:hover { background: #1d4ed8; }
    .pricing-card .btn-secondary { background: #f1f5f9; color: #2563eb; }
    .pricing-card .btn-secondary:hover { background: #e2e8f0; }
    .faq-section { padding: 80px 0; }
    .faq-section h2 { text-align: center; font-size: 2rem; font-weight: 700; color: #0f172a; margin-bottom: 48px; }
    .faq-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 24px; }
    .faq-item { padding: 24px; background: #f8fafc; border-radius: 12px; }
    .faq-item h4 { font-size: 1rem; font-weight: 700; color: #0f172a; margin-bottom: 8px; }
    .faq-item p { color: #64748b; font-size: 0.9rem; line-height: 1.7; }
    footer {
      background: #0f172a; color: #94a3b8; padding: 40px 24px; text-align: center;
    }
    footer a { color: #94a3b8; text-decoration: none; }
    footer a:hover { color: #ffffff; }
    .footer-links { display: flex; gap: 24px; justify-content: center; margin-bottom: 16px; flex-wrap: wrap; }
    .footer-links a { font-size: 0.9rem; }
    footer .copy { font-size: 0.85rem; }
    @media (max-width: 900px) {
      .pricing-grid { grid-template-columns: 1fr; }
      .pricing-card.featured { transform: none; }
      .pricing-card.featured:hover { transform: translateY(-4px); }
      .faq-grid { grid-template-columns: 1fr; }
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
      <a href="/pricing" class="active">Pricing</a>
      <a href="/contact">Contact</a>
      <a href="/contact" class="btn-primary">Get Started</a>
    </div>
  </nav>

  <div class="page-header">
    <div class="container">
      <h1>Simple, Transparent Pricing</h1>
      <p>Choose the plan that fits your team. No hidden fees, no surprise charges.</p>
    </div>
  </div>

  <section style="padding: 0 0 80px;">
    <div class="container">
      <div class="pricing-grid">
        <div class="pricing-card">
          <div class="plan-name">Starter</div>
          <div class="price">$0<span>/mo</span></div>
          <div class="description">Perfect for personal projects and learning VYX.</div>
          <ul class="features">
            <li>Up to 3 workers</li>
            <li>1 replica per worker</li>
            <li>Basic route annotations</li>
            <li>JWT authentication</li>
            <li>Community support</li>
            <li class="disabled">Advanced rate limiting</li>
            <li class="disabled">Circuit breaker</li>
          </ul>
          <a href="/contact" class="btn btn-secondary">Get Started Free</a>
        </div>
        <div class="pricing-card featured">
          <div class="pricing-badge">Most Popular</div>
          <div class="plan-name">Professional</div>
          <div class="price">$49<span>/mo</span></div>
          <div class="description">For growing teams that need reliability and performance.</div>
          <ul class="features">
            <li>Up to 10 workers</li>
            <li>5 replicas per worker</li>
            <li>Full annotation support</li>
            <li>JWT + RBAC</li>
            <li>Email support</li>
            <li>Rate limiting</li>
            <li>Circuit breaker</li>
          </ul>
          <a href="/contact" class="btn btn-primary">Start Free Trial</a>
        </div>
        <div class="pricing-card">
          <div class="plan-name">Enterprise</div>
          <div class="price">$199<span>/mo</span></div>
          <div class="description">For large-scale deployments with dedicated infrastructure.</div>
          <ul class="features">
            <li>Unlimited workers</li>
            <li>Unlimited replicas</li>
            <li>All annotations & features</li>
            <li>SSO / SAML integration</li>
            <li>Priority support (24/7)</li>
            <li>Custom rate limiting</li>
            <li>Dedicated infrastructure</li>
          </ul>
          <a href="/contact" class="btn btn-secondary">Contact Sales</a>
        </div>
      </div>
    </div>
  </section>

  <section class="faq-section" style="background: #f8fafc;">
    <div class="container">
      <h2>Frequently Asked Questions</h2>
      <div class="faq-grid">
        <div class="faq-item">
          <h4>Is there a free tier?</h4>
          <p>Yes! The Starter plan is completely free and includes up to 3 workers with 1 replica each. It's perfect for experimenting with VYX and building small projects.</p>
        </div>
        <div class="faq-item">
          <h4>Can I upgrade or downgrade anytime?</h4>
          <p>Absolutely. You can change your plan at any time. When upgrading, new features are available immediately. Downgrades take effect at the next billing cycle.</p>
        </div>
        <div class="faq-item">
          <h4>Which languages are supported?</h4>
          <p>VYX supports Go, Node.js (JavaScript/TypeScript), and Python workers. All plans include support for all three languages.</p>
        </div>
        <div class="faq-item">
          <h4>What kind of support do you offer?</h4>
          <p>Starter plan includes community support via GitHub Discussions. Professional includes email support with 24-hour response time. Enterprise gets 24/7 priority support with a dedicated account manager.</p>
        </div>
        <div class="faq-item">
          <h4>Can I self-host VYX?</h4>
          <p>Yes! VYX is designed to be self-hosted. The Enterprise plan includes dedicated infrastructure support, but you can run VYX anywhere with Docker or bare metal.</p>
        </div>
        <div class="faq-item">
          <h4>Is there a money-back guarantee?</h4>
          <p>Yes. We offer a 30-day money-back guarantee on all paid plans. If VYX doesn't meet your needs, we'll refund your first month — no questions asked.</p>
        </div>
      </div>
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

module.exports = { renderPricingPage };
