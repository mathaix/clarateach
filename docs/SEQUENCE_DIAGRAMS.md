# ClaraTeach - Sequence Diagrams

This document contains sequence diagrams for all major user flows in ClaraTeach.

---

## Table of Contents

1. [Instructor: Create Workshop](#1-instructor-create-workshop)
2. [Instructor: Start Workshop (Provision VM)](#2-instructor-start-workshop-provision-vm)
3. [Learner: Join Workshop](#3-learner-join-workshop)
4. [Learner: Connect to Workspace](#4-learner-connect-to-workspace)
5. [Learner: Reconnect After Disconnect](#5-learner-reconnect-after-disconnect)
6. [Instructor: Stop Workshop](#6-instructor-stop-workshop)
7. [Learner: Use Terminal](#7-learner-use-terminal)
8. [Learner: Use Code Editor](#8-learner-use-code-editor)
9. [Learner: View Browser Preview](#9-learner-view-browser-preview)

---

## 1. Instructor: Create Workshop

Creates a workshop record without provisioning infrastructure.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│Instructor│          │  Portal  │          │  Secret  │          │   GCP    │
│ Browser  │          │   API    │          │ Manager  │          │ Compute  │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ POST /api/workshops │                     │                     │
     │ {                   │                     │                     │
     │   name: "Claude CLI"│                     │                     │
     │   seats: 10         │                     │                     │
     │   apiKey: "sk-xxx"  │                     │                     │
     │ }                   │                     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Store API key       │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ Return secret ref   │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │                     │ Generate:                                 │
     │                     │ - workshopId: "ws-abc"                    │
     │                     │ - code: "CLAUDE-2024"                     │
     │                     │                     │                     │
     │                     │ Create placeholder VM metadata            │
     │                     │ (or store in memory/cache)                │
     │                     │─────────────────────────────────────────> │
     │                     │                     │                     │
     │ 201 Created         │                     │                     │
     │ {                   │                     │                     │
     │   workshopId: "ws-abc"                    │                     │
     │   code: "CLAUDE-2024"                     │                     │
     │   status: "created" │                     │                     │
     │ }                   │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## 2. Instructor: Start Workshop (Provision VM)

Provisions the Compute Engine VM and starts learner containers.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│Instructor│          │  Portal  │          │   GCP    │          │Workspace │
│ Browser  │          │   API    │          │ Compute  │          │    VM    │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ POST /api/workshops/ws-abc/start          │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Create VM instance  │                     │
     │                     │ - name: clarateach-ws-abc                 │
     │                     │ - type: e2-standard-8                     │
     │                     │ - metadata: apiKey, seats                 │
     │                     │ - startup-script    │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ Operation pending   │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │ 202 Accepted        │                     │                     │
     │ { status: "provisioning" }                │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │                     │      (VM boots, runs startup script)      │
     │                     │                     │                     │
     │                     │                     │ ┌─────────────────┐ │
     │                     │                     │ │ startup.sh:     │ │
     │                     │                     │ │ 1. Install Docker│ │
     │                     │                     │ │ 2. Pull images  │ │
     │                     │                     │ │ 3. Start Caddy  │ │
     │                     │                     │ │ 4. Start 10     │ │
     │                     │                     │ │    containers   │ │
     │                     │                     │ └─────────────────┘ │
     │                     │                     │                     │
     │                     │                     │ VM ready            │
     │                     │                     │<────────────────────│
     │                     │                     │                     │
     │                     │ Poll for VM status  │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ RUNNING, IP: 34.56.78.90                  │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │                     │ Create DNS record   │                     │
     │                     │ ws-abc.clarateach.io → 34.56.78.90        │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │ (Poll or WebSocket) │                     │                     │
     │ GET /api/workshops/ws-abc                 │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │ 200 OK              │                     │                     │
     │ {                   │                     │                     │
     │   status: "running" │                     │                     │
     │   endpoint: "ws-abc.clarateach.io"        │                     │
     │ }                   │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## 3. Learner: Join Workshop

Learner enters workshop code and gets assigned to a container.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│ Learner  │          │  Portal  │          │   GCP    │          │Workspace │
│ Browser  │          │   API    │          │ Compute  │          │    VM    │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ Navigate to portal.clarateach.io/join     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │ 200 OK (Join page HTML/JS)                │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ User enters code: "CLAUDE-2024"           │                     │
     │                     │                     │                     │
     │ POST /api/join      │                     │                     │
     │ { code: "CLAUDE-2024" }                   │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Find VM by label    │                     │
     │                     │ code=claude-2024    │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ VM: clarateach-ws-abc                     │
     │                     │ IP: 34.56.78.90     │                     │
     │                     │ metadata: seats-map │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │                     │ Parse seats-map: {}  │                     │
     │                     │ Assign seat 1        │                     │
     │                     │ Generate odehash: "x7k2m"                  │
     │                     │                     │                     │
     │                     │ Update VM metadata  │                     │
     │                     │ seats-map: {"x7k2m": 1}                    │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ Sign JWT:           │                     │
     │                     │ {                   │                     │
     │                     │   workshopId: "ws-abc"                    │
     │                     │   seat: 1           │                     │
     │                     │   odehash: "x7k2m"  │                     │
     │                     │   vmIp: "34.56.78.90"                     │
     │                     │ }                   │                     │
     │                     │                     │                     │
     │ 200 OK              │                     │                     │
     │ {                   │                     │                     │
     │   token: "eyJ..."   │                     │                     │
     │   endpoint: "ws-abc.clarateach.io"        │                     │
     │   odehash: "x7k2m"  │                     │                     │
     │   seat: 1           │                     │                     │
     │ }                   │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ Store token in localStorage               │                     │
     │ Show odehash to user: "x7k2m"             │                     │
     │ Redirect to workspace                     │                     │
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## 4. Learner: Connect to Workspace

After joining, learner connects to the 3-panel workspace interface.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│ Learner  │          │Workspace │          │ Learner  │          │   neko   │
│ Browser  │          │    VM    │          │Container │          │Container │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ Navigate to ws-abc.clarateach.io?token=eyJ...                   │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Validate JWT        │                     │
     │                     │ (using public key)  │                     │
     │                     │                     │                     │
     │ 200 OK (Workspace SPA)                    │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ React app loads     │                     │                     │
     │ Extracts seat=1 from token                │                     │
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │                TERMINAL CONNECTION                          │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     │ WebSocket: wss://ws-abc.clarateach.io/vm/01/terminal            │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Proxy to container-01:3001                │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ Attach to tmux      │
     │                     │                     │ session "main"      │
     │                     │                     │                     │
     │ WebSocket established                     │                     │
     │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─>│<─ ─ ─ ─ ─ ─ ─ ─ ─ ─>│                     │
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │                FILE SERVER CONNECTION                       │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     │ GET /vm/01/files/workspace                │                     │
     │────────────────────>│                     │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ { files: [...] }    │                     │
     │                     │<────────────────────│                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │                BROWSER PREVIEW CONNECTION                   │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     │ WebSocket: wss://ws-abc.clarateach.io/vm/01/browser             │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Proxy to neko-01:3003                     │
     │                     │─────────────────────────────────────────> │
     │                     │                     │                     │
     │ WebRTC negotiation  │                     │                     │
     │<─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─>│
     │                     │                     │                     │
     │ Video stream begins │                     │                     │
     │<════════════════════════════════════════════════════════════════│
     │                     │                     │                     │
     │ Workspace fully loaded                    │                     │
     │ - Terminal active   │                     │                     │
     │ - Editor showing /workspace               │                     │
     │ - Browser preview streaming               │                     │
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## 5. Learner: Reconnect After Disconnect

Learner returns after closing browser or losing connection.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│ Learner  │          │  Portal  │          │   GCP    │          │ Learner  │
│ Browser  │          │   API    │          │ Compute  │          │Container │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ (Learner closed browser earlier)          │                     │
     │                     │                     │                     │
     │                     │                     │ Container still     │
     │                     │                     │ running, tmux       │
     │                     │                     │ session persists    │
     │                     │                     │                     │
     │ Navigate to portal.clarateach.io/join     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │ 200 OK (Join page)  │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ User enters:        │                     │                     │
     │ - Code: "CLAUDE-2024"                     │                     │
     │ - Odehash: "x7k2m"  │                     │                     │
     │                     │                     │                     │
     │ POST /api/join      │                     │                     │
     │ {                   │                     │                     │
     │   code: "CLAUDE-2024"                     │                     │
     │   odehash: "x7k2m"  │                     │                     │
     │ }                   │                     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Find VM by code     │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ VM metadata:        │                     │
     │                     │ seats-map: {"x7k2m": 1, "b3n9p": 2}       │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │                     │ Lookup odehash "x7k2m"                    │
     │                     │ Found: seat 1       │                     │
     │                     │                     │                     │
     │                     │ Sign new JWT (same seat)                  │
     │                     │                     │                     │
     │ 200 OK              │                     │                     │
     │ {                   │                     │                     │
     │   token: "eyJ..."   │                     │                     │
     │   endpoint: "ws-abc.clarateach.io"        │                     │
     │   odehash: "x7k2m"  │                     │                     │
     │   seat: 1           │                     │                     │
     │ }                   │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ Redirect to workspace                     │                     │
     │────────────────────────────────────────────────────────────────>│
     │                     │                     │                     │
     │ Connect to terminal WebSocket             │                     │
     │────────────────────────────────────────────────────────────────>│
     │                     │                     │                     │
     │                     │                     │ Attach to existing  │
     │                     │                     │ tmux session        │
     │                     │                     │                     │
     │ Terminal shows previous session history   │                     │
     │ All files still present in /workspace     │                     │
     │<════════════════════════════════════════════════════════════════│
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## 6. Instructor: Stop Workshop

Instructor ends the workshop and destroys all resources.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│Instructor│          │  Portal  │          │   GCP    │          │Workspace │
│ Browser  │          │   API    │          │ Compute  │          │    VM    │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ POST /api/workshops/ws-abc/stop           │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Delete DNS record   │                     │
     │                     │ ws-abc.clarateach.io                      │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │ Delete VM instance  │                     │
     │                     │ clarateach-ws-abc   │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ ┌─────────────────┐ │
     │                     │                     │ │ VM shutting     │ │
     │                     │                     │ │ down...         │ │
     │                     │                     │ │                 │ │
     │                     │                     │ │ All containers  │ │
     │                     │                     │ │ destroyed       │ │
     │                     │                     │ │                 │ │
     │                     │                     │ │ Disk deleted    │ │
     │                     │                     │ └─────────────────┘ │
     │                     │                     │                     │
     │                     │ Operation complete  │          X          │
     │                     │<────────────────────│       (deleted)     │
     │                     │                     │                     │
     │                     │ Delete API key from │                     │
     │                     │ Secret Manager      │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │ 200 OK              │                     │                     │
     │ { status: "stopped" }                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │  Meanwhile, connected learners see:                        │ │
     │ │  - WebSocket disconnects                                   │ │
     │ │  - "Workshop ended" message                                │ │
     │ │  - Cannot reconnect (VM gone)                              │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     ▼                     ▼                     ▼
```

---

## 7. Learner: Use Terminal

Detailed flow of terminal input/output.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│ Learner  │          │ xterm.js │          │Workspace │          │  tmux    │
│ (typing) │          │(browser) │          │ Server   │          │ session  │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ Types: "claude"     │                     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ WebSocket send      │                     │
     │                     │ { type: "input",    │                     │
     │                     │   data: "claude\r" }│                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ Write to PTY        │
     │                     │                     │────────────────────>│
     │                     │                     │                     │
     │                     │                     │ tmux executes       │
     │                     │                     │ in bash session     │
     │                     │                     │                     │
     │                     │                     │ PTY output:         │
     │                     │                     │ "$ claude\r\n"      │
     │                     │                     │ "Claude CLI v1.0"   │
     │                     │                     │<────────────────────│
     │                     │                     │                     │
     │                     │ WebSocket receive   │                     │
     │                     │ { type: "output",   │                     │
     │                     │   data: "..." }     │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │ Terminal displays   │                     │                     │
     │ output              │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │  $ claude                                                  │ │
     │ │  Claude CLI v1.0                                           │ │
     │ │  > _                                                       │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## 8. Learner: Use Code Editor

File operations through Monaco editor.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│ Learner  │          │  Monaco  │          │Workspace │          │   File   │
│(editing) │          │ Editor   │          │  Server  │          │  System  │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │                    LOAD FILE TREE                          │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     │                     │ GET /files?path=/workspace               │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ readdir(/workspace) │
     │                     │                     │────────────────────>│
     │                     │                     │                     │
     │                     │                     │ [main.py, data/]    │
     │                     │                     │<────────────────────│
     │                     │                     │                     │
     │                     │ { files: [...] }    │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │ File tree displayed │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │                    OPEN FILE                               │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     │ Click on main.py   │                     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ GET /files/workspace/main.py             │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ readFile(main.py)   │
     │                     │                     │────────────────────>│
     │                     │                     │                     │
     │                     │                     │ "print('hello')"    │
     │                     │                     │<────────────────────│
     │                     │                     │                     │
     │                     │ { content: "..." }  │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │ File opens in editor│                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ ┌─────────────────────────────────────────────────────────────┐ │
     │ │                    SAVE FILE (Cmd+S)                       │ │
     │ └─────────────────────────────────────────────────────────────┘ │
     │                     │                     │                     │
     │ Edit: "print('hi')" │                     │                     │
     │ Press Cmd+S         │                     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ PUT /files/workspace/main.py             │
     │                     │ { content: "print('hi')" }               │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ writeFile(main.py)  │
     │                     │                     │────────────────────>│
     │                     │                     │                     │
     │                     │                     │ OK                  │
     │                     │                     │<────────────────────│
     │                     │                     │                     │
     │                     │ 200 OK              │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │ "Saved" indicator   │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## 9. Learner: View Browser Preview

Claude controls Playwright, learner sees the browser.

```
┌──────────┐          ┌──────────┐          ┌──────────┐          ┌──────────┐
│ Learner  │          │  Claude  │          │Playwright│          │   neko   │
│(watches) │          │   CLI    │          │ Browser  │          │ (stream) │
└────┬─────┘          └────┬─────┘          └────┬─────┘          └────┬─────┘
     │                     │                     │                     │
     │ Learner prompt:     │                     │                     │
     │ "Go to anthropic.com│                     │                     │
     │  and take screenshot"                     │                     │
     │────────────────────>│                     │                     │
     │                     │                     │                     │
     │                     │ Use playwright_navigate                   │
     │                     │ tool: anthropic.com │                     │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ Navigate to URL     │
     │                     │                     │                     │
     │                     │                     │ Page loads...       │
     │                     │                     │                     │
     │                     │                     │ Viewport updated    │
     │                     │                     │────────────────────>│
     │                     │                     │                     │
     │ WebRTC video frame  │                     │                     │
     │ shows anthropic.com │                     │                     │
     │<════════════════════════════════════════════════════════════════│
     │                     │                     │                     │
     │ (Learner sees browser                     │                     │
     │  updating in real-time)                   │                     │
     │                     │                     │                     │
     │                     │ Navigation complete │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │                     │ Use playwright_screenshot                 │
     │                     │────────────────────>│                     │
     │                     │                     │                     │
     │                     │                     │ Capture screenshot  │
     │                     │                     │ Save to /workspace/ │
     │                     │                     │ screenshot.png      │
     │                     │                     │                     │
     │                     │ Screenshot saved    │                     │
     │                     │<────────────────────│                     │
     │                     │                     │                     │
     │ Terminal shows:     │                     │                     │
     │ "Screenshot saved   │                     │                     │
     │  to screenshot.png" │                     │                     │
     │<────────────────────│                     │                     │
     │                     │                     │                     │
     │ Editor file tree    │                     │                     │
     │ updates (if watching)                     │                     │
     │                     │                     │                     │
     ▼                     ▼                     ▼                     ▼
```

---

## Connection State Machine

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Learner Session State Machine                            │
│                                                                             │
│                                                                             │
│    ┌──────────┐      join       ┌──────────┐    connect    ┌──────────┐    │
│    │          │ ───────────────>│          │ ─────────────>│          │    │
│    │   NONE   │                 │ ASSIGNED │               │CONNECTED │    │
│    │          │                 │          │               │          │    │
│    └──────────┘                 └──────────┘               └────┬─────┘    │
│                                       │                         │          │
│                                       │                    disconnect      │
│                                       │                         │          │
│                                       │                         ▼          │
│                                       │                   ┌──────────┐     │
│                                       │                   │          │     │
│                                       │    reconnect      │DISCONNECTED    │
│                                       │<──────────────────│          │     │
│                                       │                   └──────────┘     │
│                                                                 │          │
│                                                            workshop        │
│                                                             stopped        │
│                                                                 │          │
│                                                                 ▼          │
│                                                           ┌──────────┐     │
│                                                           │          │     │
│                                                           │  EXPIRED │     │
│                                                           │          │     │
│                                                           └──────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Workshop State Machine

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Workshop State Machine                                   │
│                                                                             │
│                                                                             │
│    ┌──────────┐     create     ┌──────────┐     start     ┌──────────┐     │
│    │          │ ──────────────>│          │ ─────────────>│          │     │
│    │   NONE   │                │ CREATED  │               │PROVISION-│     │
│    │          │                │          │               │   ING    │     │
│    └──────────┘                └────┬─────┘               └────┬─────┘     │
│                                     │                          │           │
│                                     │                     VM ready         │
│                                     │                          │           │
│                                     │                          ▼           │
│                                     │                    ┌──────────┐      │
│                                     │                    │          │      │
│                                     │                    │ RUNNING  │      │
│                                     │       stop         │          │      │
│                                     │<───────────────────┴────┬─────┘      │
│                                     │                         │            │
│                                     │                       stop           │
│                                     │                         │            │
│                                     ▼                         ▼            │
│                               ┌──────────┐             ┌──────────┐        │
│                               │          │             │          │        │
│                               │ DELETED  │<────────────│ STOPPING │        │
│                               │          │  VM deleted │          │        │
│                               └──────────┘             └──────────┘        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```
