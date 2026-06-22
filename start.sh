#!/usr/bin/env bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[*]${NC} $1"; }
success() { echo -e "${GREEN}[+]${NC} $1"; }
error() { echo -e "${RED}[-]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }

KVLT_STATIC_TOKENS='[{"token":"admin-token-p0","tenant_id":"t-default","scopes":["keys:manage","nodes:manage","crypto:encrypt","crypto:decrypt","datakey:generate"],"roles":["admin"]},{"token":"data-token-p0","tenant_id":"t-default","scopes":["crypto:encrypt","crypto:decrypt","datakey:generate"],"plane":"data"}]'

cleanup() {
    log "Cleaning up..."
    kill "$FRONTEND_PID" 2>/dev/null || true
    kill "$BACKEND_PID" 2>/dev/null || true
    pkill -f "key-vault" 2>/dev/null || true
    pkill -f "vite" 2>/dev/null || true
    wait 2>/dev/null || true
}
trap cleanup EXIT

kill_port() {
    local port=$1
    local pid=$(lsof -ti:"$port" 2>/dev/null || ss -tlnp | grep ":$port" | awk '{print $NF}' | cut -d',' -f1 | cut -d'=' -f2 2>/dev/null || true)
    if [ -n "$pid" ]; then
        log "Killing process on port $port (PID: $pid)"
        kill -9 "$pid" 2>/dev/null || true
        sleep 1
    fi
}

wait_port() {
    local port=$1
    local timeout=${2:-30}
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        if echo -n > /dev/tcp/localhost/"$port" 2>/dev/null; then
            return 0
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done
    return 1
}

run_smoke_test() {
    local frontend_port=${1:-5173}
    log "Running smoke tests..."
    
    local failures=0
    
    echo ""
    echo "=== TEST 1: Health Check ==="
    local result=$(curl -s http://localhost:8080/healthz)
    if echo "$result" | grep -q '"status":"ok"'; then
        success "Health check: PASSED"
    else
        error "Health check: FAILED - $result"
        failures=$((failures + 1))
    fi
    
    echo ""
    echo "=== TEST 2: Frontend Access ==="
    local html=$(curl -s -L "http://localhost:$frontend_port/ui/login")
    if [ $? -eq 0 ] && [ -n "$html" ]; then
        success "Frontend access: PASSED"
    else
        error "Frontend access: FAILED"
        failures=$((failures + 1))
    fi
    
    echo ""
    echo "=== TEST 3: Create Key ==="
    local key_result=$(curl -s -X POST \
        -H "Authorization: Bearer admin-token-p0" \
        -H "Content-Type: application/json" \
        -d '{"tenant_id":"t-default","name":"smoke-test-key","purpose":"encrypt_decrypt","policy_id":"default-v1","suite_id":"AES_256_GCM"}' \
        http://localhost:8080/ui/api/v1/keys)
    if echo "$key_result" | grep -q '"key_id"'; then
        success "Create key: PASSED"
        local key_id=$(echo "$key_result" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)
        echo "  Key ID: ${key_id:0:20}..."
    else
        error "Create key: FAILED - $key_result"
        failures=$((failures + 1))
    fi
    
    echo ""
    echo "=== TEST 4: List Keys ==="
    local list_result=$(curl -s -H "Authorization: Bearer admin-token-p0" http://localhost:8080/ui/api/v1/keys)
    if echo "$list_result" | grep -q '"keys"'; then
        success "List keys: PASSED"
    else
        error "List keys: FAILED - $list_result"
        failures=$((failures + 1))
    fi
    
    echo ""
    echo "=== TEST 5: Register Node ==="
    local node_result=$(curl -s -X POST \
        -H "Authorization: Bearer admin-token-p0" \
        -H "Content-Type: application/json" \
        -d '{"node_id":"smoke-test-node","role":"data","baseline":{"selinux_status":"enforcing","kernel_version":"5.15.0","virt_platform":"kvm","tpm2_tss_version":"3.2.0","swtpm_isolated":true}}' \
        http://localhost:8080/ui/api/v1/nodes/register)
    if echo "$node_result" | grep -q '"node_id"'; then
        success "Register node: PASSED"
    else
        error "Register node: FAILED - $node_result"
        failures=$((failures + 1))
    fi
    
    echo ""
    echo "=== TEST 6: Crypto Encrypt/Decrypt ==="
    local enc_result=$(curl -s -X POST \
        -H "Authorization: Bearer admin-token-p0" \
        -H "Content-Type: application/json" \
        -d '{"tenant_id":"t-default","key_id":"'"$key_id"'","plaintext":"aGVsbG8ga2ZsdXQ=","node_id":"smoke-test-node"}' \
        http://localhost:8080/ui/api/v1/crypto/encrypt)
    if echo "$enc_result" | grep -q '"ciphertext"'; then
        success "Encrypt: PASSED"
        success "Decrypt: SKIPPED (verified in integration tests)"
    else
        error "Encrypt: FAILED - $enc_result"
        failures=$((failures + 1))
    fi
    
    echo ""
    echo "=== TEST 7: Data Key Generation ==="
    local data_key_result=$(curl -s -X POST \
        -H "Authorization: Bearer admin-token-p0" \
        -H "Content-Type: application/json" \
        -d '{"tenant_id":"t-default","key_id":"'"$key_id"'","purpose":"test","ttl_seconds":300}' \
        http://localhost:8080/ui/api/v1/data-keys)
    if echo "$data_key_result" | grep -q '"plaintext_data_key"'; then
        success "Data key generation: PASSED"
    else
        error "Data key generation: FAILED - $data_key_result"
        failures=$((failures + 1))
    fi
    
    echo ""
    echo "======================================"
    if [ $failures -eq 0 ]; then
        success "ALL TESTS PASSED!"
        return 0
    else
        error "$failures TEST(S) FAILED!"
        return 1
    fi
}

echo ""
echo "======================================"
echo "      KVLT Key Vault - Quick Start"
echo "======================================"
echo ""

log "Step 1: Stopping existing services..."
kill_port 5173
kill_port 8080

log "Step 2: Building frontend..."
cd web/frontend
if ! npm run build 2>&1 | tail -3; then
    error "Frontend build failed!"
    exit 1
fi
success "Frontend build completed"

log "Step 3: Building backend..."
cd ../..
if ! go build -o /tmp/kvlt-server ./cmd/key-vault/ 2>&1 | tail -3; then
    error "Backend build failed!"
    exit 1
fi
success "Backend build completed"

log "Step 4: Starting backend server..."
export KVLT_STATIC_TOKENS
/tmp/kvlt-server &
BACKEND_PID=$!
sleep 3

if wait_port 8080 20; then
    success "Backend started on port 8080 (PID: $BACKEND_PID)"
else
    error "Backend failed to start!"
    exit 1
fi

log "Step 5: Starting frontend dev server..."
cd web/frontend
npm run dev > /tmp/vite.log 2>&1 &
FRONTEND_PID=$!
sleep 3

FRONTEND_PORT=5173
if ! wait_port 5173 5; then
    FRONTEND_PORT=$(grep -o 'http://localhost:[0-9]*' /tmp/vite.log | head -1 | grep -o '[0-9]*') || FRONTEND_PORT=5174
    warn "Frontend running on port $FRONTEND_PORT (auto-assigned)"
fi

if wait_port "$FRONTEND_PORT" 10; then
    success "Frontend started on port $FRONTEND_PORT (PID: $FRONTEND_PID)"
else
    error "Frontend failed to start!"
    cat /tmp/vite.log
    exit 1
fi

log "Step 6: Running smoke tests..."
if run_smoke_test "$FRONTEND_PORT"; then
    echo ""
    echo "======================================"
    success "🚀 KVLT Key Vault is ready!"
    echo "======================================"
    echo ""
    echo "  Frontend: http://localhost:$FRONTEND_PORT/ui/login"
    echo "  Backend API: http://localhost:8080/ui/api/v1/"
    echo "  Admin Token: admin-token-p0"
    echo ""
    echo "  Press Ctrl+C to stop all services"
    echo ""
else
    echo ""
    echo "======================================"
    error "Smoke tests failed!"
    echo "======================================"
    exit 1
fi

wait
