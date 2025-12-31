Start ClaraTeach development services.

Run the stack script to start all services:

```bash
./scripts/stack.sh start
```

This starts:
- Workspace containers (Docker Compose)
- Portal API on http://localhost:4000
- Frontend on http://localhost:5173

After starting, verify the services are running with `./scripts/stack.sh status`.
