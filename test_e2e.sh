#!/bin/bash
# Water Monitoring System E2E Test Suite
# Run with: bash test_e2e.sh

set -e
BASE="http://localhost:8080"
PASSED=0
FAILED=0

red() { echo -e "\033[0;31m$1\033[0m"; }
green() { echo -e "\033[0;32m$1\033[0m"; }
yellow() { echo -e "\033[0;33m$1\033[0m"; }

assert_eq() {
  if [ "$1" = "$2" ]; then
    green "✓ $3"
    ((PASSED++))
  else
    red "✗ $3 (expected '$2', got '$1')"
    ((FAILED++))
  fi
}

assert_contains() {
  if echo "$1" | grep -q "$2"; then
    green "✓ $3"
    ((PASSED++))
  else
    red "✗ $3 (expected to contain '$2' in '$1')"
    ((FAILED++))
  fi
}

assert_not_contains() {
  if ! echo "$1" | grep -q "$2"; then
    green "✓ $3"
    ((PASSED++))
  else
    red "✗ $3 (should NOT contain '$2')"
    ((FAILED++))
  fi
}

echo "======================================"
echo " Water Monitoring System E2E Tests"
echo "======================================"
echo ""

# ─── Health check ───────────────────────────────────────────────────────────
yellow ">>> Health check"
RESP=$(curl -s "$BASE/api/health")
assert_contains "$RESP" '"status":"ok"' "Health endpoint returns ok"

# ─── Auth: Login with wrong password ────────────────────────────────────────
yellow ">>> Auth: Login with wrong password"
RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"WrongPassword"}')
assert_contains "$RESP" '"error"' "Wrong password returns error"

# ─── Auth: Login with correct credentials ───────────────────────────────────
yellow ">>> Auth: Login with correct credentials"
RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"ChangeMe!123"}')
assert_contains "$RESP" '"token"' "Login returns token"
assert_contains "$RESP" '"user"' "Login returns user"
TOKEN=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
assert_not_contains "$RESP" '"password_hash"' "Login response does not leak password_hash"

# ─── Auth: /api/auth/me requires auth ───────────────────────────────────────
yellow ">>> Auth: /api/auth/me without token"
RESP=$(curl -s "$BASE/api/auth/me")
assert_contains "$RESP" '"error"' "Me endpoint requires authentication"

yellow ">>> Auth: /api/auth/me with valid token"
RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/auth/me")
assert_contains "$RESP" '"email":"admin@example.com"' "Me endpoint returns user"
assert_not_contains "$RESP" '"password_hash"' "Me response does not leak password_hash"

# ─── Admin: List users ──────────────────────────────────────────────────────
yellow ">>> Admin: List users"
RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/admin/users")
assert_contains "$RESP" '"admin@example.com"' "User list contains admin"
assert_not_contains "$RESP" '"password_hash"' "User list does not leak password_hash"

# ─── Admin: Create user ─────────────────────────────────────────────────────
yellow ">>> Admin: Create user with weak password"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/users" -d '{"email":"test@example.com","name":"Test","password":"weak"}')
assert_contains "$RESP" '"error"' "Weak password rejected"

yellow ">>> Admin: Create user with valid data"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/users" -d '{"email":"alice@example.com","name":"Alice","role":"manager","password":"StrongPass123"}')
assert_contains "$RESP" '"alice@example.com"' "User created successfully"
ALICE_ID=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')

yellow ">>> Admin: Create duplicate email"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/users" -d '{"email":"alice@example.com","name":"Alice2","password":"StrongPass123"}')
assert_contains "$RESP" '"error"' "Duplicate email rejected"

# ─── Admin: Get single user ─────────────────────────────────────────────────
yellow ">>> Admin: Get single user"
RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/admin/users/$ALICE_ID")
assert_contains "$RESP" '"alice@example.com"' "Get user returns correct user"
assert_not_contains "$RESP" '"password_hash"' "Single user does not leak password_hash"

# ─── Admin: Update user ─────────────────────────────────────────────────────
yellow ">>> Admin: Update user"
RESP=$(curl -s -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/users/$ALICE_ID" -d '{"name":"Alice Updated"}')
assert_contains "$RESP" '"Alice Updated"' "User name updated"

# ─── Admin: Reset password ──────────────────────────────────────────────────
yellow ">>> Admin: Reset user password"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/users/$ALICE_ID/reset-password" -d '{}')
assert_contains "$RESP" '"temporary_password"' "Reset returns temp password"

# ─── Auth: Login as new user ────────────────────────────────────────────────
yellow ">>> Auth: Login as Alice with new temp password"
TEMP_PW=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("temporary_password",""))')
RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"alice@example.com\",\"password\":\"$TEMP_PW\"}")
assert_contains "$RESP" '"token"' "Alice can login with temp password"
ALICE_TOKEN=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

# ─── RBAC: Alice (manager) cannot access admin endpoints ────────────────────
yellow ">>> RBAC: Manager cannot access admin user list"
RESP=$(curl -s -H "Authorization: Bearer $ALICE_TOKEN" "$BASE/api/admin/users")
assert_contains "$RESP" '"error"' "Manager blocked from admin endpoint"

yellow ">>> RBAC: Manager can read dashboard"
RESP=$(curl -s -H "Authorization: Bearer $ALICE_TOKEN" "$BASE/api/dashboard")
# Dashboard should return array or object, not error
assert_not_contains "$RESP" '"error":"authentication required"' "Manager can access dashboard"

# ─── Create viewer user for RBAC test ───────────────────────────────────────
yellow ">>> Admin: Create viewer user"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/users" -d '{"email":"viewer@example.com","name":"Viewer","role":"viewer","password":"ViewerPass123","must_change_password":false}')
assert_contains "$RESP" '"viewer@example.com"' "Viewer created"

RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"viewer@example.com","password":"ViewerPass123"}')
VIEWER_TOKEN=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

yellow ">>> RBAC: Viewer can read dashboard"
RESP=$(curl -s -H "Authorization: Bearer $VIEWER_TOKEN" "$BASE/api/dashboard")
assert_not_contains "$RESP" '"error":"authentication required"' "Viewer can read dashboard"

yellow ">>> RBAC: Viewer cannot POST readings"
RESP=$(curl -s -X POST -H "Authorization: Bearer $VIEWER_TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/readings" -d '{"meter_id":"test","value":100}')
assert_contains "$RESP" '"error"' "Viewer blocked from POST readings"

# ─── Legacy API key support ─────────────────────────────────────────────────
yellow ">>> Legacy: X-API-Key header bypasses auth (if configured)"
# Note: Default app.env has empty API key, so this should fail
RESP=$(curl -s -H "X-API-Key: test-key" "$BASE/api/dashboard")
# Without configured key, should require normal auth
assert_contains "$RESP" '"error"' "Unconfigured API key does not bypass auth"

# ─── Data endpoints ─────────────────────────────────────────────────────────
yellow ">>> Data: GET /api/sites"
RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/sites")
assert_contains "$RESP" '"rotherham"' "Sites endpoint returns sites"

yellow ">>> Data: GET /api/meters"
RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/meters")
assert_contains "$RESP" '"meter_id"' "Meters endpoint returns meters" || \
assert_contains "$RESP" '"id"' "Meters endpoint returns meters"

yellow ">>> Data: GET /api/kpis"
RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/kpis")
assert_contains "$RESP" '"site_id"' "KPIs endpoint returns data" || \
assert_not_contains "$RESP" '"error"' "KPIs endpoint succeeds"

# ─── Admin: Median fill ─────────────────────────────────────────────────────
yellow ">>> Admin: Median fill"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/median-fill" -d '{"site_id":"stocksbridge"}')
assert_contains "$RESP" '"status"' "Median fill returns status"

# ─── Admin: Activity log ────────────────────────────────────────────────────
yellow ">>> Admin: Activity log"
RESP=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/admin/activity?limit=10")
assert_contains "$RESP" '"action"' "Activity log returns entries"
assert_contains "$RESP" '"login"' "Activity log contains login events"

# ─── Admin: Delete user ─────────────────────────────────────────────────────
yellow ">>> Admin: Delete user"
RESP=$(curl -s -X DELETE -H "Authorization: Bearer $TOKEN" "$BASE/api/admin/users/$ALICE_ID")
assert_contains "$RESP" '"status":"deleted"' "User deleted"

yellow ">>> Admin: Verify deleted user cannot login"
RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"alice@example.com\",\"password\":\"$TEMP_PW\"}")
assert_contains "$RESP" '"error"' "Deleted user cannot login"

# ─── Auth: Logout ───────────────────────────────────────────────────────────
yellow ">>> Auth: Logout"
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" "$BASE/api/auth/logout")
assert_contains "$RESP" '"status":"ok"' "Logout successful"

# After logout, that specific token/session should be revoked
# (Note: we login again for final tests, so technically a new token)

# ─── Account lockout test ───────────────────────────────────────────────────
yellow ">>> Security: Account lockout after failed attempts"
# Get fresh login for admin
RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"ChangeMe!123"}')
TOKEN=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')

# Create test user for lockout
RESP=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  "$BASE/api/admin/users" -d '{"email":"locktest@example.com","name":"LockTest","password":"TestPass123","must_change_password":false}')
LOCK_ID=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("id",""))' 2>/dev/null || echo "")

if [ -n "$LOCK_ID" ]; then
  # Try 5 wrong passwords
  for i in {1..5}; do
    curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
      -d '{"email":"locktest@example.com","password":"wrong"}' > /dev/null
  done
  # 6th attempt should be locked
  RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
    -d '{"email":"locktest@example.com","password":"wrong"}')
  assert_contains "$RESP" '"locked"' "Account locked after failed attempts" || \
  assert_contains "$RESP" '"error"' "Account shows error after lockout"
fi

# ─── Password change ───────────────────────────────────────────────────────
yellow ">>> Auth: Change own password"
RESP=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"viewer@example.com","password":"ViewerPass123"}')
VT=$(echo "$RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
RESP=$(curl -s -X POST -H "Authorization: Bearer $VT" -H 'Content-Type: application/json' \
  "$BASE/api/auth/change-password" -d '{"current_password":"ViewerPass123","new_password":"NewViewer456"}')
assert_contains "$RESP" '"status"' "Password change successful"

# Old session should be revoked after password change
yellow ">>> Auth: Old session revoked after password change"
RESP=$(curl -s -H "Authorization: Bearer $VT" "$BASE/api/auth/me")
assert_contains "$RESP" '"error"' "Old session revoked after password change"

# ─── Summary ────────────────────────────────────────────────────────────────
echo ""
echo "======================================"
echo " Test Results"
echo "======================================"
green "Passed: $PASSED"
if [ $FAILED -gt 0 ]; then
  red "Failed: $FAILED"
  exit 1
else
  green "All tests passed!"
fi
