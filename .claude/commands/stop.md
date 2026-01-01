Stop ClaraTeach development services.

Press `Ctrl+C` in the terminal running each service, or:

```bash
# Kill backend (port 8080)
lsof -ti:8080 | xargs kill -9 2>/dev/null || echo "Backend not running"

# Kill frontend (port 5173)
lsof -ti:5173 | xargs kill -9 2>/dev/null || echo "Frontend not running"
```
