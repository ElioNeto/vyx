#!/usr/bin/env bash
# install-deps.sh
# Instala as dependências dos scripts do boilerplate-opencode.
# Uso: bash scripts/install-deps.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log()  { echo -e "${BLUE}[boilerplate]${NC} $*"; }
ok()   { echo -e "${GREEN}[ok]${NC} $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }
fail() { echo -e "${RED}[error]${NC} $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Verificar pré-requisitos
# ---------------------------------------------------------------------------
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
      warn "$label não encontrado (opcional). Alguns recursos podem não funcionar."
    fi
  fi
}

check_cmd "node"   "Node.js" "true"
check_cmd "npm"    "npm"     "true"
check_cmd "docker" "Docker"  "false"

# Verificar versão do Node.js >= 20
NODE_MAJOR=$(node -e "process.stdout.write(process.version.slice(1).split('.')[0])")
if [ "$NODE_MAJOR" -lt 20 ]; then
  fail "Node.js >= 20 é obrigatório. Versão atual: $(node --version)"
fi
ok "Node.js v${NODE_MAJOR}.x (>= 20)"

# ---------------------------------------------------------------------------
# Instalar dependências dos scripts
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
log "Instalando dependências em $SCRIPT_DIR..."

cd "$SCRIPT_DIR"
npm install --silent
ok "Dependências instaladas (yaml, tsx, typescript)"

# ---------------------------------------------------------------------------
# Verificar npx tsx
# ---------------------------------------------------------------------------
if ! npx tsx --version &>/dev/null; then
  fail "npx tsx falhou após instalação. Verifique o npm."
fi
ok "npx tsx disponível"

# ---------------------------------------------------------------------------
# Verificar scripts
# ---------------------------------------------------------------------------
log "Verificando scripts..."

for script in workflow-agent.ts check-todos.ts; do
  if [ -f "$SCRIPT_DIR/$script" ]; then
    ok "$script encontrado"
  else
    fail "$script não encontrado em $SCRIPT_DIR/"
  fi
done

# ---------------------------------------------------------------------------
# Instalação concluída
# ---------------------------------------------------------------------------
echo ""
echo -e "${GREEN}================================================${NC}"
echo -e "${GREEN}  boilerplate-opencode: dependências prontas!  ${NC}"
echo -e "${GREEN}================================================${NC}"
echo ""
log "Comandos disponíveis:"
echo "  npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml [job] [--dry-run]"
echo "  npx tsx scripts/check-todos.ts [.task-state.json]"
echo ""

if ! command -v docker &>/dev/null; then
  warn "Docker não encontrado. O workflow-agent requer Docker para executar os jobs."
  warn "Instale em: https://docs.docker.com/get-docker/"
fi
