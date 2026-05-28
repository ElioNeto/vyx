/**
 * Dashboard page component for the VYX Dashboard.
 *
 * @Page(/dashboard)
 * @Auth(roles: ["user", "admin"])
 *
 * Shows analytics overview, user greeting, navigation sidebar with
 * links to all pages. Renders mock data directly in the HTML:
 * - Stats cards (total users, active sessions, page views, conversion rate)
 * - CSS bar charts for monthly revenue
 * - User table
 */

// In a full React-based SSR setup, this component would be compiled
// and rendered server-side. In this example, the worker.js file
// handles all rendering directly with template strings.
//
// This file serves as documentation for the annotation scanner.

export function renderDashboardPage(req) {
  const userName = (req.claims && req.claims.name) || 'User';
  const userRole = (req.claims && req.claims.role) || 'user';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Dashboard — VYX</title>
</head>
<body>
  <nav id="sidebar">
    <div class="logo">VYX<span>Dash</span></div>
    <a href="/dashboard" class="active">📊 Dashboard</a>
    <a href="/settings">⚙️ Settings</a>
  </nav>
  <main>
    <header>
      <h1>Dashboard</h1>
      <div class="user-info">${userName} (${userRole})</div>
    </header>
    <section id="content">
      <div class="stats-grid">
        <div class="stat-card">
          <span class="label">Total Users</span>
          <span class="value">1,250</span>
          <span class="change up">+12.5%</span>
        </div>
        <div class="stat-card">
          <span class="label">Active Sessions</span>
          <span class="value">438</span>
          <span class="change up">+8.2%</span>
        </div>
        <div class="stat-card">
          <span class="label">Page Views</span>
          <span class="value">312.4K</span>
          <span class="change up">+23.1%</span>
        </div>
        <div class="stat-card">
          <span class="label">Conversion Rate</span>
          <span class="value">3.42%</span>
          <span class="change down">-0.8%</span>
        </div>
      </div>

      <div class="card">
        <h2>Monthly Revenue (CSS Bar Chart)</h2>
        <div class="chart">
          <div class="bar" style="height:45%"><span>$45K</span></div>
          <div class="bar" style="height:52%"><span>$52K</span></div>
          <div class="bar" style="height:48%"><span>$48K</span></div>
          <div class="bar" style="height:61%"><span>$61K</span></div>
          <div class="bar" style="height:58%"><span>$58K</span></div>
          <div class="bar" style="height:65%"><span>$65K</span></div>
        </div>
        <div class="chart-labels">
          <span>Jan</span><span>Feb</span><span>Mar</span>
          <span>Apr</span><span>May</span><span>Jun</span>
        </div>
      </div>

      <div class="card">
        <h2>Users</h2>
        <table>
          <thead>
            <tr><th>Name</th><th>Email</th><th>Role</th></tr>
          </thead>
          <tbody>
            <tr><td>Admin User</td><td>admin@dashboard.local</td><td>admin</td></tr>
            <tr><td>Regular User</td><td>user@dashboard.local</td><td>user</td></tr>
            <tr><td>Alice Johnson</td><td>alice@example.com</td><td>user</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </main>
</body>
</html>`;
}
