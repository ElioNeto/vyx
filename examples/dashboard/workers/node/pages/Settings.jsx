/**
 * Settings page component for the VYX Dashboard.
 *
 * @Page(/settings)
 * @Auth(roles: ["user", "admin"])
 *
 * Settings page with profile info and preferences form
 * (theme, language, timezone, notifications).
 */

export function renderSettingsPage(req) {
  const userName = (req.claims && req.claims.name) || 'User';
  const userEmail = (req.claims && req.claims.email) || '';
  const userRole = (req.claims && req.claims.role) || 'user';

  return `
    <div class="card">
      <h2>Profile</h2>
      <div class="form-group">
        <label>Name</label>
        <input type="text" value="${userName}" disabled>
      </div>
      <div class="form-group">
        <label>Email</label>
        <input type="email" value="${userEmail}" disabled>
      </div>
      <div class="form-group">
        <label>Role</label>
        <input type="text" value="${userRole}" disabled>
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
      <button class="btn btn-primary" id="save-settings-btn">Save Changes</button>
    </div>

    <script>
      document.getElementById('save-settings-btn').onclick = function() {
        var btn = this;
        btn.disabled = true;
        btn.textContent = 'Saving...';
        var data = {
          theme: document.getElementById('theme').value,
          language: document.getElementById('language').value,
          timezone: document.getElementById('timezone').value,
          notifications_enabled: document.getElementById('notifications').checked,
        };
        var token = localStorage.getItem('vyx_token');
        fetch('/api/users/me/settings', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
          body: JSON.stringify(data),
        }).then(function(r) {
          if (r.ok) { alert('Settings saved!'); }
          else { r.json().then(function(j) { alert('Error: ' + (j.error || 'unknown')); }); }
        }).catch(function(e) {
          alert('Error: ' + e.message);
        }).finally(function() {
          btn.disabled = false;
          btn.textContent = 'Save Changes';
        });
      };
    </script>
  `;
}
