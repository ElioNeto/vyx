/**
 * Login page component for the VYX Dashboard.
 *
 * @Page(/login)
 * @Auth(roles: ["guest"])
 *
 * Renders a clean login form that POSTs to /api/auth/login.
 * On success, stores the JWT token in localStorage and redirects
 * to the dashboard.
 */

// In a full React-based SSR setup, this component would be compiled
// and rendered server-side. In this example, the worker.js file
// handles all rendering directly with template strings.
//
// This file serves as documentation for the annotation scanner.

export function renderLoginPage(req) {
  const error = (req.query && req.query.error) ? req.query.error : '';

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Login — VYX Dashboard</title>
</head>
<body>
  <div id="login-root">
    <h1>Welcome back</h1>
    <p>Sign in to your dashboard account</p>
    <form action="/api/auth/login" method="POST">
      <label>Username <input type="text" name="username" /></label>
      <label>Password <input type="password" name="password" /></label>
      <button type="submit">Sign in</button>
    </form>
    ${error ? `<div class="error">${error}</div>` : ''}
  </div>
</body>
</html>`;
}
