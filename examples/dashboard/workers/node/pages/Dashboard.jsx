/**
 * Dashboard page component for the VYX Dashboard.
 *
 * @Page(/dashboard)
 * @Auth(roles: ["user", "admin"])
 *
 * Dark-themed analytics dashboard with stats cards, CSS bar chart,
 * and user table. Renders within the main layout (sidebar + header).
 */

export function renderDashboardPage(req) {
  const userName = (req.claims && req.claims.name) || 'User';
  const userRole = (req.claims && req.claims.role) || 'user';

  return `
    <div class="welcome-box">
      Welcome back, <strong>${userName}</strong>! Here is your dashboard overview.
    </div>

    <div class="stats-grid">
      <div class="stat-card">
        <div class="label">Total Users</div>
        <div class="value">1,250</div>
        <div class="change up">+12.5%</div>
      </div>
      <div class="stat-card">
        <div class="label">Active Sessions</div>
        <div class="value">438</div>
        <div class="change up">+8.2%</div>
      </div>
      <div class="stat-card">
        <div class="label">Page Views</div>
        <div class="value">312.4K</div>
        <div class="change up">+23.1%</div>
      </div>
      <div class="stat-card">
        <div class="label">Conversion Rate</div>
        <div class="value">3.42%</div>
        <div class="change down">-0.8%</div>
      </div>
    </div>

    <div class="card">
      <h2>Monthly Revenue</h2>
      <div class="chart-bar-container">
        <div class="chart-bar" style="height:100px"><span class="bar-value">$45K</span></div>
        <div class="chart-bar" style="height:115px"><span class="bar-value">$52K</span></div>
        <div class="chart-bar" style="height:106px"><span class="bar-value">$48K</span></div>
        <div class="chart-bar" style="height:135px"><span class="bar-value">$61K</span></div>
        <div class="chart-bar" style="height:128px"><span class="bar-value">$58K</span></div>
        <div class="chart-bar" style="height:145px"><span class="bar-value">$65K</span></div>
      </div>
      <div class="chart-labels">
        <span>Jan</span><span>Feb</span><span>Mar</span>
        <span>Apr</span><span>May</span><span>Jun</span>
      </div>
    </div>

    <div class="card">
      <h2>Recent Users</h2>
      <table>
        <thead>
          <tr><th>Name</th><th>Email</th><th>Role</th></tr>
        </thead>
        <tbody>
          <tr><td>Admin User</td><td>admin@dashboard.local</td><td><span class="badge badge-admin">admin</span></td></tr>
          <tr><td>Regular User</td><td>user@dashboard.local</td><td><span class="badge badge-user">user</span></td></tr>
          <tr><td>Alice Johnson</td><td>alice@example.com</td><td><span class="badge badge-user">user</span></td></tr>
        </tbody>
      </table>
    </div>
  `;
}
