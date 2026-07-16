#!/usr/bin/env bash
# ─── VYX Dashboard — Test Validation Pipeline ───────────────────────────────
# Validates end-to-end flows: API, SSR pages, authentication, and log output.
#
# Usage:
#   ./scripts/validate.sh              # validate against running server
#   ./scripts/validate.sh --dev        # start dev server, validate, then stop
#
# Exit code: 0 = all checks pass, 1 = any check failed
# ──────────────────────────────────────────────────────────────────────────────
# No set -e; we handle errors manually for better diagnostics.

BASE_URL="${VYX_BASE_URL:-http://localhost:8080}"
PASS=0
FAIL=0
LOG_FILE=$(mktemp /tmp/vyx-validate-XXXXX.log)
PID=""

cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    echo ""
    echo "→ Stopping vyx dev server (PID $PID)..."
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -f "$LOG_FILE"
}
trap cleanup EXIT

# ─── Helpers ─────────────────────────────────────────────────────────────────

check() {
  local name="$1" expected="$2" actual="$3"
  if echo "$actual" | grep -q "$expected"; then
    echo "  ✅ PASS: $name"
    ((PASS++))
  else
    echo "  ❌ FAIL: $name"
    echo "     expected: $expected"
    echo "     got:      $(echo "$actual" | head -c 200)"
    ((FAIL++))
  fi
}

check_log() {
  local name="$1" pattern="$2"
  if grep -q "$pattern" "$LOG_FILE"; then
    echo "  ✅ PASS: $name"
    ((PASS++))
  else
    echo "  ❌ FAIL: $name (pattern not found in logs)"
    ((FAIL++))
  fi
}

api() {
  curl -s -o /dev/null -w "%{http_code}" "$@"
}

api_body() {
  curl -s "$@"
}

# ─── Start server (if --dev) ─────────────────────────────────────────────────

if [ "${1:-}" = "--dev" ]; then
  SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
  echo "=== Starting vyx dev server ==="
  export VYX_SKIP_RUNTIME=1
  export JWT_SECRET="${JWT_SECRET:-supersecret_supersecret_supersecret_1234}"

  # Kill any existing server on port 8080
  lsof -ti:8080 2>/dev/null | xargs kill -9 2>/dev/null || true
  sleep 1

  # Start server in background, capturing logs
  cd "$SCRIPT_DIR"
  ../../vyx dev > "$LOG_FILE" 2>&1 &
  PID=$!

  # Wait for server to be ready
  for i in $(seq 1 30); do
    if curl -s -o /dev/null "$BASE_URL/login" 2>/dev/null; then
      echo "  → Server ready after ${i}s"
      break
    fi
    if [ "$i" -eq 30 ]; then
      echo "  ❌ Server failed to start within 30s"
      cat "$LOG_FILE"
      exit 1
    fi
    sleep 1
  done
  echo ""
else
  # Use existing server
  echo "=== Validating against $BASE_URL ==="
  echo ""

  # Check server is reachable
  if ! curl -s -o /dev/null "$BASE_URL/login" 2>/dev/null; then
    echo "❌ Server not reachable at $BASE_URL"
    echo "   Start it with: cd examples/dashboard && ../../vyx dev"
    exit 1
  fi
fi

# ─── 1. Login Page (SSR HTML) ────────────────────────────────────────────────

echo "═══ 1. Login Page ═══"
HTML=$(api_body "$BASE_URL/login")
check "Login page returns HTML" "<!DOCTYPE html>" "$(echo "$HTML" | head -1)"
check "Login page has form" "Sign in" "$HTML"

# ─── 2. Login API (JSON) ─────────────────────────────────────────────────────

echo ""
echo "═══ 2. Login API (JSON) ═══"
LOGIN=$(api_body -X POST "$BASE_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
check "Login returns token" "token" "$LOGIN"
check "Login returns user" '"name":"admin"' "$LOGIN"

# Extract token
TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null || echo "")
if [ -z "$TOKEN" ]; then
  echo "  ⚠️  Could not extract token, trying alternative method..."
  TOKEN=$(echo "$LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
fi
echo "  → Token: ${TOKEN:0:40}..."

# ─── 3. Login API (form-encoded — simulates browser without JS) ──────────────

echo ""
echo "═══ 3. Login API (Form-Encoded — browser without JS) ═══"
LOGIN2_HEADERS=$(curl -s -D - -X POST "$BASE_URL/api/auth/login" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'username=admin&password=admin123' -o /dev/null 2>&1)
check "Form login redirects (302)" "302" "$(echo "$LOGIN2_HEADERS" | head -1)"
check "Form login sets cookie" "Set-Cookie.*vyx_token" "$LOGIN2_HEADERS"
check "Form login redirects to dashboard" "Location.*/dashboard" "$LOGIN2_HEADERS"

# ─── 4. Dashboard (SSR HTML with auth) ───────────────────────────────────────

echo ""
echo "═══ 4. Dashboard (Authenticated) ═══"
DASH=$(api_body -H "Authorization: Bearer $TOKEN" "$BASE_URL/dashboard")
check "Dashboard returns HTML" "Dashboard" "$DASH"
check "Dashboard has stats" "Total Users" "$DASH"
check "Dashboard has chart" "chart-bar" "$DASH"
check "Dashboard has users table" "Admin User" "$DASH"

# ─── 5. Dashboard without auth → 401 ───────────────────────────────────────

echo ""
echo "═══ 5. Dashboard (Unauthenticated) ═══"
CODE=$(api -w "%{http_code}" "$BASE_URL/dashboard")
check "Dashboard rejects anonymous" "401" "$CODE"

# ─── 6. Users API ────────────────────────────────────────────────────────────

echo ""
echo "═══ 6. Users API ═══"
USERS=$(api_body -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/users")
check "List users returns data" "total" "$USERS"
check "Admin user exists" "admin@dashboard.local" "$USERS"

# ─── 7. Current User Profile ─────────────────────────────────────────────────

echo ""
echo "═══ 7. Current User Profile ═══"
ME=$(api_body -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/users/1")
check "Get user returns profile" "admin" "$ME"

# ─── 8. Settings Page (SSR HTML) ─────────────────────────────────────────────

echo ""
echo "═══ 8. Settings Page ═══"
SETTINGS=$(api_body -H "Authorization: Bearer $TOKEN" "$BASE_URL/settings")
check "Settings returns HTML" "Settings" "$SETTINGS"
check "Settings has form" "Theme" "$SETTINGS"

# ─── 9. Log Validation ───────────────────────────────────────────────────────

echo ""
echo "═══ 9. Log Validation ═══"
if [ -s "$LOG_FILE" ]; then
  check_log "Server started" "HTTP server listening"
  check_log "Login request processed" "POST /api/auth/login"
  check_log "Guest auth for login" "\"guest\""
  check_log "Workers are running" "worker.*running"
else
  echo "  ⚠️  Log file empty — logs not available (server was already running)"
  echo "     Restart with --dev to capture logs"
fi

# ─── Results ─────────────────────────────────────────────────────────────────

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  Results: $PASS passed, $FAIL failed"
echo "═══════════════════════════════════════════════════════════════"
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
