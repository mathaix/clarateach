Check the status of ClaraTeach development services.

```bash
# Check if backend is running (port 8080)
curl -s http://localhost:8080/api/health && echo " Backend OK" || echo "Backend not running"

# Check if frontend is running (port 5173)
curl -s http://localhost:5173 >/dev/null && echo "Frontend OK" || echo "Frontend not running"
```

Services:
- Backend: http://localhost:8080
- Frontend: http://localhost:5173
