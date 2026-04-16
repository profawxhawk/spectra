#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

cleanup() {
  echo ""
  echo -e "${YELLOW}Shutting down...${NC}"
  [[ -n "${BACKEND_PID:-}" ]] && kill "$BACKEND_PID" 2>/dev/null && echo "  Stopped backend"
  [[ -n "${FRONTEND_PID:-}" ]] && kill "$FRONTEND_PID" 2>/dev/null && echo "  Stopped frontend"
  echo -e "${GREEN}Done.${NC}"
}
trap cleanup EXIT INT TERM

# ── 1. Check prerequisites ──────────────────────────────────────────
echo -e "${CYAN}[1/5] Checking prerequisites...${NC}"

command -v go   >/dev/null 2>&1 || { echo -e "${RED}Error: Go is not installed${NC}"; exit 1; }
command -v node >/dev/null 2>&1 || { echo -e "${RED}Error: Node.js is not installed${NC}"; exit 1; }
command -v docker >/dev/null 2>&1 || { echo -e "${RED}Error: Docker is not installed${NC}"; exit 1; }

echo "  go    $(go version | awk '{print $3}')"
echo "  node  $(node -v)"
echo "  docker $(docker --version | awk '{print $3}' | tr -d ',')"

# ── 2. Start infrastructure ─────────────────────────────────────────
echo -e "${CYAN}[2/5] Starting Postgres + Redis + MinIO...${NC}"

docker compose up -d --wait 2>/dev/null || docker-compose up -d 2>/dev/null
echo "  Postgres  localhost:5432"
echo "  Redis     localhost:6379"
echo "  MinIO     localhost:9000"

# ── 3. Build backend ────────────────────────────────────────────────
echo -e "${CYAN}[3/5] Building Go backend...${NC}"

go build -o bin/spectra ./cmd/spectra
echo "  Built bin/spectra"

# ── 4. Start backend ────────────────────────────────────────────────
echo -e "${CYAN}[4/5] Starting backend on :8080...${NC}"

mkdir -p /tmp/spectra/data /tmp/spectra/index
./bin/spectra serve --config spectra.yaml &
BACKEND_PID=$!

# Wait for backend to be ready
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then
    echo -e "  Backend ${GREEN}ready${NC}"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo -e "  ${RED}Backend failed to start${NC}"
    exit 1
  fi
  sleep 1
done

# ── 5. Start frontend ───────────────────────────────────────────────
echo -e "${CYAN}[5/5] Starting frontend on :5173...${NC}"

cd web
if [ ! -d node_modules ]; then
  npm install --silent
fi
npx vite --host 2>/dev/null &
FRONTEND_PID=$!
cd "$ROOT"

sleep 2

# ── Ready ────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  Spectra is running!${NC}"
echo ""
echo -e "  Frontend:  ${CYAN}http://localhost:5173${NC}"
echo -e "  Backend:   ${CYAN}http://localhost:8080${NC}"
echo -e "  Health:    ${CYAN}http://localhost:8080/healthz${NC}"
echo ""
echo -e "  ${YELLOW}Ingest sample data:${NC}"
echo '  curl -X POST http://localhost:8080/v1/spans \\'
echo '    -H "Content-Type: application/json" \\'
echo '    -d '"'"'{"spans":[{"span_id":"s1","trace_id":"t1","name":"gpt4_call","kind":"llm","start_time":"2024-01-15T10:30:00Z","input":"Hello","output":"Hi there","metrics":{"latency_ms":1500,"prompt_tokens":10,"completion_tokens":20}}]}'"'"''
echo ""
echo -e "  ${YELLOW}Press Ctrl+C to stop${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Keep running until Ctrl+C
wait
