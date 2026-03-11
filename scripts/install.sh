#!/usr/bin/env bash
# vyx — Getting Started installer for Linux and macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/ElioNeto/vyx/main/scripts/install.sh | bash

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

VYX_MIN_GO="1.22"
VYX_MIN_NODE="20"
VYX_MIN_PYTHON="3.11"

log()  { echo -e "${CYAN}[vyx]${NC} $*"; }
ok()   { echo -e "${GREEN}[vyx] ✔${NC} $*"; }
warn() { echo -e "${YELLOW}[vyx] ⚠${NC} $*"; }
fail() { echo -e "${RED}[vyx] ✖${NC} $*" >&2; exit 1; }

check_cmd() { command -v "$1" &>/dev/null; }

version_gte() {
  local installed="$1" required="$2"
  local major_i minor_i major_r minor_r
  major_i=$(echo "$installed" | cut -d. -f1)
  minor_i=$(echo "$installed" | cut -d. -f2)
  major_r=$(echo "$required"  | cut -d. -f1)
  minor_r=$(echo "$required"  | cut -d. -f2)
  [[ "$major_i" -gt "$major_r" ]] && return 0
  [[ "$major_i" -eq "$major_r" && "$minor_i" -ge "$minor_r" ]] && return 0
  return 1
}

echo ""
echo -e "${CYAN}  ██╗   ██╗██╗   ██╗██╗  ██╗${NC}"
echo -e "${CYAN}  ██║   ██║╚██╗ ██╔╝╚██╗██╔╝${NC}"
echo -e "${CYAN}  ██║   ██║ ╚████╔╝  ╚███╔╝ ${NC}"
echo -e "${CYAN}  ╚██████╔╝  ╚██╔╝   ██╔██╗ ${NC}"
echo -e "${CYAN}   ╚═════╝    ╚═╝   ╚═╝  ╚═╝${NC}"
echo -e "  Polyglot Full-Stack Framework"
echo ""

# ── 1. Detect OS ─────────────────────────────────────────────────────────────
OS="$(uname -s)"
log "Detected OS: $OS"

# ── 2. Check prerequisites ────────────────────────────────────────────────────
log "Checking prerequisites..."

# Go
if check_cmd go; then
  GO_VER=$(go version | grep -oE '[0-9]+\.[0-9]+' | head -1)
  if version_gte "$GO_VER" "$VYX_MIN_GO"; then
    ok "Go $GO_VER found"
  else
    fail "Go $VYX_MIN_GO+ required (found $GO_VER). Install: https://go.dev/dl/"
  fi
else
  fail "Go not found. Install Go $VYX_MIN_GO+: https://go.dev/dl/"
fi

# Node.js
if check_cmd node; then
  NODE_VER=$(node --version | tr -d 'v' | cut -d. -f1)
  if [[ "$NODE_VER" -ge "$VYX_MIN_NODE" ]]; then
    ok "Node.js v$NODE_VER found"
  else
    fail "Node.js $VYX_MIN_NODE+ required. Install: https://nodejs.org/"
  fi
else
  warn "Node.js not found (required for Node workers and SSR). Install: https://nodejs.org/"
fi

# Python
if check_cmd python3; then
  PY_VER=$(python3 --version 2>&1 | grep -oE '[0-9]+\.[0-9]+' | head -1)
  if version_gte "$PY_VER" "$VYX_MIN_PYTHON"; then
    ok "Python $PY_VER found"
  else
    warn "Python $VYX_MIN_PYTHON+ recommended (found $PY_VER). Install: https://python.org/"
  fi
else
  warn "Python 3 not found (required for Python workers). Install: https://python.org/"
fi

# Git
if check_cmd git; then
  ok "Git found"
else
  fail "Git not found. Install: https://git-scm.com/"
fi

# ── 3. Clone repository ───────────────────────────────────────────────────────
DEST="${1:-vyx-project}"

if [[ -d "$DEST" ]]; then
  warn "Directory '$DEST' already exists. Skipping clone."
else
  log "Cloning vyx into ./$DEST ..."
  git clone --depth 1 https://github.com/ElioNeto/vyx.git "$DEST"
  ok "Repository cloned into ./$DEST"
fi

cd "$DEST"

# ── 4. Build the core ─────────────────────────────────────────────────────────
log "Building vyx core..."
mkdir -p bin
cd core
go mod download
go build -o ../bin/vyx ./cmd/vyx
cd ..
ok "vyx binary built at $(pwd)/bin/vyx"

# ── 5. Add to PATH hint ───────────────────────────────────────────────────────
BIN_PATH="$(pwd)/bin"

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  vyx is ready! 🚀${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Add vyx to your PATH:"
echo ""
echo -e "  ${YELLOW}export PATH=\"$BIN_PATH:\$PATH\"${NC}"
echo ""
echo "  To persist it, add the line above to ~/.bashrc or ~/.zshrc:"
echo ""
echo -e "  ${YELLOW}echo 'export PATH=\"$BIN_PATH:\$PATH\"' >> ~/.zshrc${NC}"
echo ""
echo "  Then start a new project:"
echo ""
echo -e "  ${YELLOW}vyx new my-app${NC}"
echo -e "  ${YELLOW}cd my-app${NC}"
echo -e "  ${YELLOW}vyx dev${NC}"
echo ""
