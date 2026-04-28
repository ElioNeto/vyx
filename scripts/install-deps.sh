#!/usr/bin/env bash
# install-deps.sh
# Instala as dependências dos scripts do vyx (workflow-agent e check-todos).
# Uso: bash scripts/install-deps.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${BLUE}[vyx]${NC} $*"; }
ok()   { echo -e "${GREEN}[ok]${NC} $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }
fail() { echo -e "${RED}[error]${NC} $*" >&2; exit 1; }

log "Verificando pré-requisitos..."

check_cmd() {
  local cmd="$1" label="${2:-$1}" required="${3:-true}"
  if command -v "$cmd" &>/dev/null; then
    local version
    version=$("$cmd" --version 2>&1 | head -1 || true)
    ok "$label encontrado: $version"
  else
    if [ "$required" = "true" ]; then
      fail "$label não encontrado. Instale antes de continuar."
    else
      warn "$label não encontrado (opcional)."
    fi
  fi
}

check_cmd "node"   "Node.js"   "true"
check_cmd "npm"    "npm"       "true"
check_cmd "go"     "Go"        "false"
check_cmd "docker" "Docker"    "false"
check_cmd "golangci-lint" "golangci-lint" "false"
check_cmd "govulncheck"   "govulncheck"   "false"

NODE_MAJOR=$(node -e "process.stdout.write(process.version.slice(1).split('.')[0])")
if [ "$NODE_MAJOR" -lt 20 ]; then
  fail "Node.js >= 20 é obrigatório. Versão atual: $(node --version)"
fi
ok "Node.js v${NODE_MAJOR}.x (>= 20)"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
log "Instalando dependências em $SCRIPT_DIR..."

cd "$SCRIPT_DIR"
npm install --silent
ok "Dependências instaladas (yaml, tsx, typescript)"

if ! npx tsx --version &>/dev/null; then
  fail "npx tsx falhou após instalação."
fi
ok "npx tsx disponível"

for script in workflow-agent.ts check-todos.ts; do
  if [ -f "$SCRIPT_DIR/$script" ]; then
    ok "$script encontrado"
  else
    fail "$script não encontrado em $SCRIPT_DIR/"
  fi
done

echo ""
echo -e "${GREEN}==============================${NC}"
echo -e "${GREEN}  vyx: dependências prontas!  ${NC}"
echo -e "${GREEN}==============================${NC}"
echo ""
log "Comandos disponíveis:"
echo "  npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml [job] [--dry-run]"
echo "  npx tsx scripts/check-todos.ts [.task-state.json]"
echo ""

if ! command -v docker &>/dev/null; then
  warn "Docker não encontrado. O workflow-agent requer Docker para executar jobs."
  warn "Instale em: https://docs.docker.com/get-docker/"
fi

if ! command -v golangci-lint &>/dev/null; then
  warn "golangci-lint não encontrado."
  warn "Instale: https://golangci-lint.run/usage/install/"
fi

if ! command -v govulncheck &>/dev/null; then
  warn "govulncheck não encontrado."
  warn "Instale: go install golang.org/x/vuln/cmd/govulncheck@latest"
fi
