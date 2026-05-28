/**
 * Home Page - rendered by node:ssr worker
 *
 * @Page(/)
 * @Auth(roles: ["guest"])
 *
 * This page shows:
 * - Hero section with headline and CTA
 * - Features overview grid (3 features)
 * - Testimonials section
 * - Newsletter signup CTA
 */

'use strict';

function renderHomePage() {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>VYX - Build Full-Stack Apps Faster</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
      color: #1a202c; line-height: 1.6; background: #ffffff;
    }
    .container { max-width: 1200px; margin: 0 auto; padding: 0 24px; }
    /* Navigation */
    nav {
      display: flex; align-items: center; justify-content: space-between;
      padding: 16px 24px; background: #ffffff; border-bottom: 1px solid #e2e8f0;
      position: sticky; top: 0; z-index: 100;
    }
    .nav-brand { font-size: 1.5rem; font-weight: 800; color: #2563eb; text-decoration: none; }
    .nav-links { display: flex; gap: 24px; align-items: center; }
    .nav-links a { text-decoration: none; color: #4a5568; font-weight: 500; font-size: 0.95rem; transition: color 0.2s; }
    .nav-links a:hover { color: #2563eb; }
    .nav-links .btn-primary {
      background: #2563eb; color: #ffffff !important; padding: 8px 20px;
      border-radius: 6px; font-weight: 600; transition: background 0.2s;
    }
    .nav-links .btn-primary:hover { background: #1d4ed8; }
    /* Hero */
    .hero {
      text-align: center; padding: 100px 24px 80px;
      background: linear-gradient(180deg, #eff6ff 0%, #ffffff 100%);
    }
    .hero h1 { font-size: 3.2rem; font-weight: 800; color: #0f172a; margin-bottom: 16px; line-height: 1.2; }
    .hero h1 span { color: #2563eb; }
    .hero p { font-size: 1.2rem; color: #64748b; max-width: 640px; margin: 0 auto 36px; }
    .hero-buttons { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
    .hero-buttons .btn { padding: 14px 32px; border-radius: 8px; font-size: 1rem; font-weight: 600; text-decoration: none; display: inline-block; }
    .hero-buttons .btn-primary { background: #2563eb; color: #ffffff; }
    .hero-buttons .btn-secondary { background: #ffffff; color: #2563eb; border: 2px solid #2563eb; }
    .hero-buttons .btn-primary:hover { background: #1d4ed8; }
    .hero-buttons .btn-secondary:hover { background: #eff6ff; }
    /* Sections */
    section { padding: 80px 0; }
    .section-title { text-align: center; margin-bottom: 56px; }
    .section-title h2 { font-size: 2.2rem; font-weight: 700; color: #0f172a; margin-bottom: 12px; }
    .section-title p { color: #64748b; font-size: 1.1rem; max-width: 560px; margin: 0 auto; }
    /* Features Grid */
    .features-grid {
      display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
      gap: 24px; margin-top: 16px;
    }
    .feature-card {
      background: #ffffff; border: 1px solid #e2e8f0; border-radius: 12px;
      padding: 32px; transition: box-shadow 0.3s, transform 0.3s;
    }
    .feature-card:hover { box-shadow: 0 8px 30px rgba(0,0,0,0.08); transform: translateY(-2px); }
    .feature-icon {
      width: 48px; height: 48px; background: #eff6ff; border-radius: 12px;
      display: flex; align-items: center; justify-content: center;
      font-size: 1.5rem; margin-bottom: 16px;
    }
    .feature-card h3 { font-size: 1.2rem; font-weight: 700; margin-bottom: 8px; color: #0f172a; }
    .feature-card p { color: #64748b; font-size: 0.95rem; line-height: 1.7; }
    /* Testimonials */
    .testimonials { background: #f8fafc; }
    .testimonials-grid {
      display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 24px;
    }
    .testimonial-card {
      background: #ffffff; border-radius: 12px; padding: 28px;
      border: 1px solid #e2e8f0;
    }
    .testimonial-card .quote { color: #475569; font-style: italic; margin-bottom: 16px; line-height: 1.7; }
    .testimonial-card .author { font-weight: 600; color: #0f172a; }
    .testimonial-card .role { color: #94a3b8; font-size: 0.85rem; }
    /* Newsletter */
    .newsletter {
      background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
      border-radius: 16px; padding: 56px; text-align: center; color: #ffffff;
      margin-top: 16px;
    }
    .newsletter h3 { font-size: 1.8rem; font-weight: 700; margin-bottom: 8px; }
    .newsletter p { opacity: 0.9; margin-bottom: 24px; font-size: 1rem; }
    .newsletter-form { display: flex; gap: 8px; max-width: 480px; margin: 0 auto; }
    .newsletter-form input {
      flex: 1; padding: 12px 16px; border-radius: 8px; border: none;
      font-size: 1rem; outline: none;
    }
    .newsletter-form button {
      background: #0f172a; color: #ffffff; border: none; padding: 12px 28px;
      border-radius: 8px; font-weight: 600; cursor: pointer; font-size: 1rem;
      transition: background 0.2s;
    }
    .newsletter-form button:hover { background: #1e293b; }
    /* Footer */
    footer {
      background: #0f172a; color: #94a3b8; padding: 40px 24px; text-align: center;
    }
    footer a { color: #94a3b8; text-decoration: none; }
    footer a:hover { color: #ffffff; }
    .footer-links { display: flex; gap: 24px; justify-content: center; margin-bottom: 16px; flex-wrap: wrap; }
    .footer-links a { font-size: 0.9rem; }
    footer .copy { font-size: 0.85rem; }
    /* Responsive */
    @media (max-width: 768px) {
      .hero h1 { font-size: 2.2rem; }
      .nav-links { gap: 12px; }
      .newsletter-form { flex-direction: column; }
      .features-grid { grid-template-columns: 1fr; }
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
      <a href="/contact">Contact</a>
      <a href="/contact" class="btn-primary">Get Started</a>
    </div>
  </nav>

  <section class="hero">
    <div class="container">
      <h1>Build Full-Stack Apps<br><span>10x Faster</span></h1>
      <p>VYX is a polyglot framework where Go, Node.js, and Python workers communicate via Unix Domain Sockets. Write microservices in the language that fits best, all orchestrated by a high-performance Go core.</p>
      <div class="hero-buttons">
        <a href="/contact" class="btn btn-primary">Get Started Free</a>
        <a href="/features" class="btn btn-secondary">Learn More</a>
      </div>
    </div>
  </section>

  <section>
    <div class="container">
      <div class="section-title">
        <h2>Why Choose VYX?</h2>
        <p>Three powerful pillars that make VYX the ideal framework for modern applications.</p>
      </div>
      <div class="features-grid">
        <div class="feature-card">
          <div class="feature-icon">⚡</div>
          <h3>Polyglot Workers</h3>
          <p>Write each microservice in the best language for the job — Go for performance-critical APIs, Node.js for SSR, Python for data science. All workers speak the same IPC protocol.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">🔌</div>
          <h3>Annotation-Based Routing</h3>
          <p>Declare routes with simple annotations like @Route and @Auth directly in your source code. The VYX scanner generates the route map automatically — no manual configuration needed.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">🔒</div>
          <h3>Enterprise Security</h3>
          <p>Built-in JWT authentication, JSON Schema validation, rate limiting, circuit breakers, and payload size limits. Production-ready security out of the box.</p>
        </div>
      </div>
    </div>
  </section>

  <section class="testimonials">
    <div class="container">
      <div class="section-title">
        <h2>Trusted by Developers</h2>
        <p>What our early adopters are saying about VYX.</p>
      </div>
      <div class="testimonials-grid">
        <div class="testimonial-card">
          <div class="quote">"VYX's annotation-based routing is a game changer. We migrated from a monolithic Express app and cut our codebase in half."</div>
          <div class="author">Sarah Chen</div>
          <div class="role">Lead Engineer, TechFlow</div>
        </div>
        <div class="testimonial-card">
          <div class="quote">"The polyglot worker model lets my team use the right tool for each job without the microservices headache. Brilliant!"</div>
          <div class="author">Marcus Johnson</div>
          <div class="role">CTO, DataPulse Inc.</div>
        </div>
        <div class="testimonial-card">
          <div class="quote">"Setup took 10 minutes. The IPC protocol is fast and the built-in circuit breaker saved us during a traffic spike."</div>
          <div class="author">Priya Patel</div>
          <div class="role">Senior Developer, CloudNine</div>
        </div>
      </div>
    </div>
  </section>

  <section>
    <div class="container">
      <div class="newsletter">
        <h3>Stay Updated</h3>
        <p>Get the latest VYX news, tutorials, and releases delivered to your inbox.</p>
        <form class="newsletter-form" action="/api/newsletter" method="POST">
          <input type="email" name="email" placeholder="Enter your email" required>
          <button type="submit">Subscribe</button>
        </form>
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

module.exports = { renderHomePage };
