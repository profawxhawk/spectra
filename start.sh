#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

STARTED_CONTAINERS=()

cleanup() {
  echo ""
  echo -e "${YELLOW}Shutting down...${NC}"
  [[ -n "${BACKEND_PID:-}" ]] && kill "$BACKEND_PID" 2>/dev/null && echo "  Stopped backend"
  [[ -n "${FRONTEND_PID:-}" ]] && kill "$FRONTEND_PID" 2>/dev/null && echo "  Stopped frontend"
  for c in "${STARTED_CONTAINERS[@]}"; do
    docker stop "$c" >/dev/null 2>&1 && docker rm "$c" >/dev/null 2>&1 && echo "  Stopped $c"
  done
  echo -e "${GREEN}Done.${NC}"
}
trap cleanup EXIT INT TERM

port_open() { nc -z localhost "$1" 2>/dev/null; }

# Parse HTTP port from config
HTTP_PORT=$(grep 'http_addr' spectra.yaml 2>/dev/null | sed 's/.*":\([0-9]*\)".*/\1/' || echo "8090")
HTTP_PORT=${HTTP_PORT:-8090}

# ── 1. Check prerequisites ──────────────────────────────────────────
echo -e "${CYAN}[1/5] Checking prerequisites...${NC}"

command -v go   >/dev/null 2>&1 || { echo -e "${RED}Error: Go is not installed${NC}"; exit 1; }
command -v node >/dev/null 2>&1 || { echo -e "${RED}Error: Node.js is not installed${NC}"; exit 1; }
command -v docker >/dev/null 2>&1 || { echo -e "${RED}Error: Docker is not installed${NC}"; exit 1; }

echo "  go     $(go version | awk '{print $3}')"
echo "  node   $(node -v)"
echo "  docker $(docker --version | awk '{print $3}' | tr -d ',')"

# ── 2. Start infrastructure (only what's missing) ───────────────────
echo -e "${CYAN}[2/5] Starting infrastructure...${NC}"

PG_PORT=5433
REDIS_PORT=6379

if port_open $PG_PORT; then
  echo -e "  Postgres  :$PG_PORT ${GREEN}already running${NC}"
else
  echo "  Starting Postgres on :$PG_PORT..."
  docker run -d --name spectra-postgres \
    -e POSTGRES_DB=spectra -e POSTGRES_USER=spectra -e POSTGRES_PASSWORD=spectra \
    -p $PG_PORT:5432 postgres:16-alpine >/dev/null
  STARTED_CONTAINERS+=(spectra-postgres)
  for i in $(seq 1 30); do
    port_open $PG_PORT && break
    sleep 1
  done
  echo -e "  Postgres  :$PG_PORT ${GREEN}started${NC}"
fi

if port_open $REDIS_PORT; then
  echo -e "  Redis     :$REDIS_PORT ${GREEN}already running${NC}"
else
  echo "  Starting Redis on :$REDIS_PORT..."
  docker run -d --name spectra-redis \
    -p $REDIS_PORT:6379 redis:7-alpine >/dev/null
  STARTED_CONTAINERS+=(spectra-redis)
  for i in $(seq 1 15); do
    port_open $REDIS_PORT && break
    sleep 1
  done
  echo -e "  Redis     :$REDIS_PORT ${GREEN}started${NC}"
fi

# ── 3. Build backend ────────────────────────────────────────────────
echo -e "${CYAN}[3/5] Building Go backend...${NC}"

go build -o bin/spectra ./cmd/spectra
echo "  Built bin/spectra"

# ── 4. Start backend ────────────────────────────────────────────────
echo -e "${CYAN}[4/5] Starting backend on :${HTTP_PORT}...${NC}"

if port_open "$HTTP_PORT"; then
  echo -e "  ${RED}Port $HTTP_PORT already in use. Change http_addr in spectra.yaml${NC}"
  exit 1
fi

mkdir -p /tmp/spectra/data /tmp/spectra/index
./bin/spectra serve --config spectra.yaml &
BACKEND_PID=$!

for i in $(seq 1 30); do
  if curl -sf http://localhost:${HTTP_PORT}/healthz >/dev/null 2>&1; then
    echo -e "  Backend   :${HTTP_PORT} ${GREEN}ready${NC}"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo -e "  ${RED}Backend failed to start. Check logs above.${NC}"
    exit 1
  fi
  sleep 1
done

# ── 5. Start frontend ───────────────────────────────────────────────
echo -e "${CYAN}[5/5] Starting frontend on :5173...${NC}"

cd web
if [ ! -d node_modules ]; then
  echo "  Installing npm dependencies..."
  npm install --silent
fi
SPECTRA_BACKEND_PORT=$HTTP_PORT npx vite --host 2>/dev/null &
FRONTEND_PID=$!
cd "$ROOT"

sleep 2
echo -e "  Frontend  :5173 ${GREEN}ready${NC}"

# ── Ready ────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  Spectra is running!${NC}"
echo ""
echo -e "  Frontend:  ${CYAN}http://localhost:5173${NC}"
echo -e "  Backend:   ${CYAN}http://localhost:${HTTP_PORT}${NC}"
echo -e "  Health:    ${CYAN}http://localhost:${HTTP_PORT}/healthz${NC}"
echo ""
echo -e "  ${YELLOW}Ingest sample data:${NC}"
echo "  curl -X POST http://localhost:${HTTP_PORT}/v1/spans \\"
echo '    -H "Content-Type: application/json" \'
echo '    -d '"'"'{"spans":[{"span_id":"s1","trace_id":"t1","name":"gpt4_call","kind":"llm","start_time":"2024-01-15T10:30:00Z","input":"Hello","output":"Hi there","metrics":{"latency_ms":1500,"prompt_tokens":10,"completion_tokens":20}}]}'"'"''
echo ""
echo -e "  ${YELLOW}Press Ctrl+C to stop${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

wait
