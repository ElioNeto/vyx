/**
 * Settings page component for the VYX Dashboard.
 *
 * @Page(/settings)
 * @Auth(roles: ["user", "admin"])
 *
 * Settings page with theme selector, notification toggle, language
 * selector, and timezone picker. Displays current user info.
 * Settings are saved via PUT /api/users/:id/settings.
 */

// In a full React-based SSR setup, this component would be compiled
// and rendered server-side. In this example, the worker.js file
// handles all rendering directly with template strings.
//
// This file serves as documentation for the annotation scanner.

export function renderSettingsPage(req) {
  const userName = (req.claims && req.claims.name) || 'User';
  const userEmail = (req.claims && req.claims.email) || '';
  const userRole = (req.claims && req.claims.role) || 'user';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Settings — VYX</title>
</head>
<body>
  <nav id="sidebar">
    <div class="logo">VYX<span>Dash</span></div>
    <a href="/dashboard">📊 Dashboard</a>
    <a href="/settings" class="active">⚙️ Settings</a>
  </nav>
  <main>
    <header>
      <h1>Settings</h1>
      <div class="user-info">${userName} (${userRole})</div>
    </header>
    <section id="content">
      <div class="card">
        <h2>Profile</h2>
        <p><strong>Name:</strong> ${userName}</p>
        <p><strong>Email:</strong> ${userEmail}</p>
        <p><strong>Role:</strong> ${userRole}</p>
      </div>

      <div class="card">
        <h2>Preferences</h2>
        <form id="settings-form">
          <label>Theme
            <select name="theme">
              <option value="system">System</option>
              <option value="light">Light</option>
              <option value="dark">Dark</option>
            </select>
          </label>
          <label>Language
            <select name="language">
              <option value="en">English</option>
              <option value="pt">Português</option>
              <option value="es">Español</option>
            </select>
          </label>
          <label>Timezone
            <select name="timezone">
              <option value="UTC">UTC</option>
              <option value="America/New_York">America/New_York</option>
              <option value="America/Sao_Paulo">America/Sao_Paulo</option>
            </select>
          </label>
          <label>
            <input type="checkbox" name="notifications_enabled" checked />
            Enable notifications
          </label>
          <button type="submit">Save changes</button>
        </form>
      </div>
    </section>
  </main>
</body>
</html>`;
}
