/**
 * Login page component for the VYX Dashboard.
 *
 * @Page(/login)
 * @Auth(roles: ["guest"])
 *
 * Renders a clean, dark-themed login form that POSTs to /api/auth/login.
 * Works both with JavaScript (JSON API) and without (form-encoded redirect).
 */

export function renderLoginPage(req) {
  const error = (req.query && req.query.error) ? req.query.error : '';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Login — VYX Dashboard</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" rel="stylesheet">
</head>
<body class="login-page">
  <div class="login-card">
    <h1>Welcome back</h1>
    <p>Sign in to your dashboard account</p>
    <div class="info-box">
      <strong>Demo credentials:</strong><br>
      Admin: <strong>admin</strong> / <strong>admin123</strong><br>
      User: (any name) / password
    </div>
    <form method="POST" action="/api/auth/login" onsubmit="event.preventDefault(); login(this);">
      <div class="form-group">
        <label>Username</label>
        <input type="text" name="username" placeholder="Enter your username" required minlength="3">
      </div>
      <div class="form-group">
        <label>Password</label>
        <input type="password" name="password" placeholder="••••••••" required minlength="6">
      </div>
      <button type="submit" class="btn btn-primary btn-block">Sign in</button>
      ${error ? `<div class="error-msg">${error}</div>` : ''}
    </form>
  </div>
  <script>
    async function login(form) {
      const data = { username: form.username.value, password: form.password.value };
      try {
        const res = await fetch('/api/auth/login', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data)
        });
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
