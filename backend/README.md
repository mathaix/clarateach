# ClaraTeach Unified Backend

This is the new Go-based backend for ClaraTeach. It replaces the legacy Node.js Portal and Bash orchestration scripts.

## Features

*   **Unified Binary:** Contains API, Database, and Orchestrator.
*   **Persistence:** Uses SQLite (by default) to save state, preventing data loss on restart.
*   **Robust Orchestration:** Uses the native Docker SDK instead of shell scripts.
*   **Dynamic Proxy:** Automatically routes traffic (WebSockets/HTTP) to the correct learner container based on the subdomain.

## Architecture

```
[ Browser ] -> [ Caddy (SSL) ] -> [ Go Backend ]
                                        |
                                        +--> [ API (REST) ]
                                        +--> [ SQLite DB ]
                                        +--> [ Orchestrator ] -> [ Docker / Firecracker ]
                                        +--> [ Proxy ] -> [ Learner Container ]
```

## Running (Local Dev)

You can run this without installing Go by using the helper script:

```bash
./scripts/run_backend_docker.sh
```

This will:
1.  Download a Go environment.
2.  Mount the `backend/` code.
3.  Connect to your local Docker daemon.
4.  Start the server on port `8080`.

## API Endpoints

*   `GET /api/workshops`: List workshops
*   `POST /api/workshops`: Create a workshop
*   `POST /api/join`: Join a workshop (returns JWT and Endpoint)

## Transition Plan

1.  Stop the old Node.js portal (`npm stop` in `portal/`).
2.  Start this backend.
3.  Update your `Caddyfile` to point to `localhost:8080`.
