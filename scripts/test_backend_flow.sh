#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}>>> Starting ClaraTeach Backend Test Flow...${NC}"

# 1. Clean up previous runs
echo -e "${BLUE}>>> Cleaning up old containers...${NC}"
docker rm -f $(docker ps -aq --filter "label=clarateach.type=learner-workspace") 2>/dev/null || true
docker rm -f $(docker ps -aq --filter "label=clarateach.type=learner-browser") 2>/dev/null || true
docker rm -f $(docker ps -aq --filter "label=clarateach.component=backend") 2>/dev/null || true
docker network rm $(docker network ls -q --filter "label=clarateach.type=workshop-network") 2>/dev/null || true
rm -f backend/clarateach.db

# 2. Start Backend in Background
echo -e "${BLUE}>>> Starting Go Backend (Local)...${NC}"
./scripts/run_backend_local.sh > backend.log 2>&1 &
BACKEND_PID=$!

echo "Waiting for backend to start..."
for i in {1..30}; do
  if grep -q "ClaraTeach Backend running" backend.log; then
    echo -e "${GREEN}Backend is UP!${NC}"
    break
  fi
  sleep 1
done

# 3. Create Workshop
echo -e "${BLUE}>>> Creating Workshop 'Demo Workshop'...${NC}"
WORKSHOP_RESP=$(curl -s -X POST http://localhost:8080/api/workshops \
  -H "Content-Type: application/json" \
  -d '{"name": "Demo Workshop", "seats": 5, "api_key": "sk-mock-key"}')

WORKSHOP_ID=$(echo $WORKSHOP_RESP | grep -o '"id":"[^"]*' | cut -d'"' -f4)
WORKSHOP_CODE=$(echo $WORKSHOP_RESP | grep -o '"code":"[^"]*' | cut -d'"' -f4)

echo -e "${GREEN}Created Workshop!${NC}"
echo "  ID:   $WORKSHOP_ID"
echo "  Code: $WORKSHOP_CODE"

echo ""
echo -e "${YELLOW}>>> READY FOR UI TESTING${NC}"
echo "To see the Workspace UI, you must run the frontend in a separate terminal:"

echo "  cd frontend"
  echo "  npm run dev"
echo "Then open this URL in your browser to join:"
echo -e "${BLUE}http://localhost:5173/join?code=${WORKSHOP_CODE}${NC}"

echo "The backend is running with PID $BACKEND_PID."
echo "Press Ctrl+C to stop the backend."

# Wait for user to Ctrl+C
wait $BACKEND_PID