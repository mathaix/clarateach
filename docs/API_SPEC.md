# ClaraTeach - API Specification

This document defines the API endpoints for ClaraTeach.

---

## Base URLs

| Environment | URL |
|-------------|-----|
| Production | `https://api.clarateach.io` |
| Development | `http://localhost:3000` |

---

## Authentication

### Portal API (Admin)

Future: Instructor authentication via OAuth/JWT.

MVP: No authentication required for instructor endpoints.

### Workspace API

Requests to workspace endpoints require a JWT token:

```
Authorization: Bearer <token>
```

The token is obtained via the `/api/join` endpoint.

---

## Common Response Formats

### Success Response

```json
{
  "data": { ... }
}
```

### Error Response

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `NOT_FOUND` | 404 | Resource not found |
| `INVALID_INPUT` | 400 | Request validation failed |
| `ALREADY_RUNNING` | 400 | Workshop already running |
| `NOT_RUNNING` | 400 | Workshop not running |
| `NO_SEATS` | 400 | Workshop is full |
| `SESSION_NOT_FOUND` | 404 | Session/odehash not found |
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `FORBIDDEN` | 403 | Access denied |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Portal API Endpoints

### Health Check

#### `GET /api/health`

Check if the API is running.

**Response:**

```json
{
  "status": "ok"
}
```

---

### Workshops

#### `GET /api/workshops`

List all workshops.

**Response:**

```json
{
  "workshops": [
    {
      "id": "ws-abc123",
      "name": "Claude CLI Basics",
      "code": "CLAUDE-XY9Z",
      "seats": 10,
      "status": "running",
      "created_at": "2024-01-15T10:30:00Z",
      "vm_name": "clarateach-ws-abc123",
      "vm_ip": "34.56.78.90",
      "endpoint": "https://34.56.78.90"
    }
  ]
}
```

---

#### `GET /api/workshops/:id`

Get a single workshop by ID.

**Parameters:**

| Name | Type | Location | Description |
|------|------|----------|-------------|
| `id` | string | path | Workshop ID |

**Response:**

```json
{
  "workshop": {
    "id": "ws-abc123",
    "name": "Claude CLI Basics",
    "code": "CLAUDE-XY9Z",
    "seats": 10,
    "status": "running",
    "created_at": "2024-01-15T10:30:00Z",
    "vm_name": "clarateach-ws-abc123",
    "vm_ip": "34.56.78.90",
    "endpoint": "https://34.56.78.90"
  }
}
```

**Errors:**

| Code | Description |
|------|-------------|
| `NOT_FOUND` | Workshop not found |

---

#### `POST /api/workshops`

Create a new workshop.

**Request Body:**

```json
{
  "name": "Claude CLI Basics",
  "seats": 10,
  "api_key": "sk-ant-api03-..."
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Workshop name (1-100 chars) |
| `seats` | integer | Yes | Number of seats (1-10) |
| `api_key` | string | Yes | Claude API key |

**Response (201 Created):**

```json
{
  "workshop": {
    "id": "ws-abc123",
    "name": "Claude CLI Basics",
    "code": "CLAUDE-XY9Z",
    "seats": 10,
    "status": "created",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

**Errors:**

| Code | Description |
|------|-------------|
| `INVALID_INPUT` | Validation failed (seats out of range, missing fields) |

---

#### `POST /api/workshops/:id/start`

Start a workshop (provision VM).

**Parameters:**

| Name | Type | Location | Description |
|------|------|----------|-------------|
| `id` | string | path | Workshop ID |

**Response (202 Accepted):**

```json
{
  "workshop": {
    "id": "ws-abc123",
    "name": "Claude CLI Basics",
    "code": "CLAUDE-XY9Z",
    "seats": 10,
    "status": "provisioning",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

The workshop transitions through states:
1. `provisioning` - VM is being created
2. `running` - VM is ready, learners can join

Poll `GET /api/workshops/:id` to check when status becomes `running`.

**Errors:**

| Code | Description |
|------|-------------|
| `NOT_FOUND` | Workshop not found |
| `ALREADY_RUNNING` | Workshop is already running |

---

#### `POST /api/workshops/:id/stop`

Stop a workshop (destroy VM).

**Parameters:**

| Name | Type | Location | Description |
|------|------|----------|-------------|
| `id` | string | path | Workshop ID |

**Response:**

```json
{
  "success": true
}
```

**Errors:**

| Code | Description |
|------|-------------|
| `NOT_FOUND` | Workshop not found |
| `NOT_RUNNING` | Workshop is not running |

---

### Sessions

#### `POST /api/join`

Join a workshop as a learner.

**Request Body:**

```json
{
  "code": "CLAUDE-XY9Z",
  "odehash": "x7k2m"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `code` | string | Yes | Workshop code |
| `odehash` | string | No | Reconnect code (for returning learners) |

**Response:**

```json
{
  "token": "eyJhbGciOiJSUzI1NiIs...",
  "endpoint": "https://34.56.78.90",
  "odehash": "x7k2m",
  "seat": 1
}
```

| Field | Description |
|-------|-------------|
| `token` | JWT token for workspace access |
| `endpoint` | Workspace VM URL |
| `odehash` | Reconnect code (save this!) |
| `seat` | Assigned seat number |

**Errors:**

| Code | Description |
|------|-------------|
| `NOT_FOUND` | Workshop not found |
| `NOT_RUNNING` | Workshop has not started |
| `NO_SEATS` | Workshop is full |
| `SESSION_NOT_FOUND` | Odehash not found (reconnect failed) |

---

## Workspace API Endpoints

All workspace endpoints are accessed via the workspace VM endpoint:

```
https://<vm-ip>/vm/<seat>/...
```

### Terminal

#### `WebSocket /vm/:seat/terminal`

Connect to terminal via WebSocket.

**URL Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `seat` | string | Seat number (e.g., "01") |

**Authentication:**

JWT token required (in query param or header).

**Client → Server Messages:**

```typescript
// Send input to terminal
{
  "type": "input",
  "data": "ls -la\n"
}

// Resize terminal
{
  "type": "resize",
  "cols": 80,
  "rows": 24
}
```

**Server → Client Messages:**

```typescript
// Terminal output
{
  "type": "output",
  "data": "total 24\ndrwxr-xr-x  4 learner learner 4096 Jan 15 10:30 .\n..."
}
```

---

### Files

#### `GET /vm/:seat/files`

List files in a directory.

**URL Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `seat` | string | Seat number |

**Query Parameters:**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| `path` | string | `/workspace` | Directory path |

**Response:**

```json
{
  "files": [
    {
      "name": "main.py",
      "path": "/workspace/main.py",
      "is_directory": false,
      "size": 1234,
      "modified_at": "2024-01-15T10:30:00Z"
    },
    {
      "name": "data",
      "path": "/workspace/data",
      "is_directory": true,
      "size": 4096,
      "modified_at": "2024-01-15T10:25:00Z"
    }
  ]
}
```

---

#### `GET /vm/:seat/files/:path`

Read a file's contents.

**URL Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `seat` | string | Seat number |
| `path` | string | File path (e.g., `workspace/main.py`) |

**Response:**

```json
{
  "content": "print('Hello, world!')\n",
  "encoding": "utf-8"
}
```

For binary files:

```json
{
  "content": "iVBORw0KGgoAAAANSUhEUgAA...",
  "encoding": "base64"
}
```

**Errors:**

| Code | Description |
|------|-------------|
| `NOT_FOUND` | File not found |
| `FORBIDDEN` | Path outside workspace |

---

#### `PUT /vm/:seat/files/:path`

Write content to a file.

**URL Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `seat` | string | Seat number |
| `path` | string | File path |

**Request Body:**

```json
{
  "content": "print('Updated!')\n",
  "encoding": "utf-8"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `content` | string | - | File content |
| `encoding` | string | `utf-8` | `utf-8` or `base64` |

**Response:**

```json
{
  "success": true
}
```

---

#### `DELETE /vm/:seat/files/:path`

Delete a file or directory.

**URL Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `seat` | string | Seat number |
| `path` | string | File/directory path |

**Response:**

```json
{
  "success": true
}
```

---

### Browser Preview

#### `WebSocket /vm/:seat/browser`

Connect to browser preview via WebSocket/WebRTC.

This endpoint is used for neko browser streaming.

**URL Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `seat` | string | Seat number |

**Protocol:**

Uses neko's WebRTC signaling protocol. See [neko documentation](https://github.com/m1k1o/neko).

Key messages:

```typescript
// Client connects
{ "event": "connect" }

// Server sends WebRTC offer
{
  "event": "offer",
  "data": { /* RTCSessionDescription */ }
}

// Client sends answer
{
  "event": "answer",
  "data": { /* RTCSessionDescription */ }
}

// ICE candidates
{
  "event": "candidate",
  "data": { /* RTCIceCandidate */ }
}
```

---

## JWT Token Format

Tokens are RS256-signed JWTs with the following payload:

```json
{
  "workshop_id": "ws-abc123",
  "container_id": "c-01",
  "seat": 1,
  "odehash": "x7k2m",
  "vm_ip": "34.56.78.90",
  "exp": 1705401600
}
```

| Field | Description |
|-------|-------------|
| `workshop_id` | Workshop ID |
| `container_id` | Container ID (c-01, c-02, etc.) |
| `seat` | Seat number (1-10) |
| `odehash` | Reconnect code |
| `vm_ip` | Workspace VM IP address |
| `exp` | Token expiry (Unix timestamp) |

---

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| `POST /api/workshops` | 10 per hour per IP |
| `POST /api/join` | 60 per hour per IP |
| All other endpoints | 1000 per hour per IP |

---

## WebSocket Protocols

### Terminal WebSocket

```
wss://<vm-ip>/vm/<seat>/terminal?token=<jwt>
```

**Subprotocol:** None (JSON messages)

**Ping/Pong:** Server sends ping every 30 seconds. Client should respond with pong.

**Reconnection:** Client should implement exponential backoff (1s, 2s, 4s, 8s, max 30s).

### Browser WebSocket

```
wss://<vm-ip>/vm/<seat>/browser?token=<jwt>
```

**Subprotocol:** neko

**See:** neko WebRTC documentation for message formats.

---

## OpenAPI Schema

```yaml
openapi: 3.0.3
info:
  title: ClaraTeach API
  version: 0.1.0
  description: API for ClaraTeach learning platform

servers:
  - url: https://api.clarateach.io
    description: Production
  - url: http://localhost:3000
    description: Development

paths:
  /api/health:
    get:
      summary: Health check
      responses:
        '200':
          description: API is healthy
          content:
            application/json:
              schema:
                type: object
                properties:
                  status:
                    type: string
                    example: ok

  /api/workshops:
    get:
      summary: List workshops
      responses:
        '200':
          description: List of workshops
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ListWorkshopsResponse'
    post:
      summary: Create workshop
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateWorkshopRequest'
      responses:
        '201':
          description: Workshop created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CreateWorkshopResponse'

  /api/workshops/{id}:
    get:
      summary: Get workshop
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Workshop details
          content:
            application/json:
              schema:
                type: object
                properties:
                  workshop:
                    $ref: '#/components/schemas/Workshop'
        '404':
          description: Workshop not found

  /api/workshops/{id}/start:
    post:
      summary: Start workshop
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '202':
          description: Workshop starting
          content:
            application/json:
              schema:
                type: object
                properties:
                  workshop:
                    $ref: '#/components/schemas/Workshop'

  /api/workshops/{id}/stop:
    post:
      summary: Stop workshop
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Workshop stopped
          content:
            application/json:
              schema:
                type: object
                properties:
                  success:
                    type: boolean

  /api/join:
    post:
      summary: Join workshop
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/JoinRequest'
      responses:
        '200':
          description: Joined successfully
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/JoinResponse'

components:
  schemas:
    Workshop:
      type: object
      properties:
        id:
          type: string
          example: ws-abc123
        name:
          type: string
          example: Claude CLI Basics
        code:
          type: string
          example: CLAUDE-XY9Z
        seats:
          type: integer
          example: 10
        status:
          type: string
          enum: [created, provisioning, running, stopping, stopped]
        created_at:
          type: string
          format: date-time
        vm_name:
          type: string
        vm_ip:
          type: string
        endpoint:
          type: string

    CreateWorkshopRequest:
      type: object
      required:
        - name
        - seats
        - api_key
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 100
        seats:
          type: integer
          minimum: 1
          maximum: 10
        api_key:
          type: string
          minLength: 10

    CreateWorkshopResponse:
      type: object
      properties:
        workshop:
          $ref: '#/components/schemas/Workshop'

    ListWorkshopsResponse:
      type: object
      properties:
        workshops:
          type: array
          items:
            $ref: '#/components/schemas/Workshop'

    JoinRequest:
      type: object
      required:
        - code
      properties:
        code:
          type: string
          minLength: 4
          maxLength: 20
        odehash:
          type: string
          minLength: 5
          maxLength: 5

    JoinResponse:
      type: object
      properties:
        token:
          type: string
        endpoint:
          type: string
        odehash:
          type: string
        seat:
          type: integer

    FileInfo:
      type: object
      properties:
        name:
          type: string
        path:
          type: string
        is_directory:
          type: boolean
        size:
          type: integer
        modified_at:
          type: string
          format: date-time

    APIError:
      type: object
      properties:
        error:
          type: object
          properties:
            code:
              type: string
            message:
              type: string
```
