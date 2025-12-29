# ClaraTeach - TypeScript Implementation

This document describes the TypeScript implementation of ClaraTeach.

---

## Project Structure

```
clarateach/
├── package.json                    # Monorepo root
├── pnpm-workspace.yaml
├── turbo.json
├── tsconfig.base.json
│
├── apps/
│   ├── portal/                     # Admin API (Cloud Run)
│   ├── web/                        # React Frontend
│   └── workspace/                  # Container workspace server
│
├── packages/
│   ├── types/                      # Shared TypeScript types
│   ├── config/                     # Shared configs
│   └── utils/                      # Shared utilities
│
├── containers/
│   ├── workspace/                  # Learner container image
│   └── neko/                       # Browser streaming image
│
├── infrastructure/                 # Terraform
│
└── cli/                            # Instructor CLI
```

---

## Package Configuration

### Root `package.json`

```json
{
  "name": "clarateach",
  "private": true,
  "packageManager": "pnpm@8.15.0",
  "scripts": {
    "dev": "turbo dev",
    "build": "turbo build",
    "test": "turbo test",
    "lint": "turbo lint",
    "typecheck": "turbo typecheck",
    "clean": "turbo clean && rm -rf node_modules"
  },
  "devDependencies": {
    "@types/node": "^20.10.0",
    "turbo": "^2.0.0",
    "typescript": "^5.3.0"
  }
}
```

### `pnpm-workspace.yaml`

```yaml
packages:
  - 'apps/*'
  - 'packages/*'
  - 'cli'
```

### `turbo.json`

```json
{
  "$schema": "https://turbo.build/schema.json",
  "globalDependencies": ["**/.env.*local"],
  "pipeline": {
    "build": {
      "dependsOn": ["^build"],
      "outputs": ["dist/**", ".next/**"]
    },
    "dev": {
      "cache": false,
      "persistent": true
    },
    "test": {
      "dependsOn": ["build"]
    },
    "lint": {},
    "typecheck": {
      "dependsOn": ["^build"]
    },
    "clean": {
      "cache": false
    }
  }
}
```

### `tsconfig.base.json`

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022"],
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  }
}
```

---

## Shared Types Package

### `packages/types/package.json`

```json
{
  "name": "@clarateach/types",
  "version": "0.1.0",
  "private": true,
  "main": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "scripts": {
    "build": "tsc",
    "typecheck": "tsc --noEmit"
  }
}
```

### `packages/types/src/index.ts`

```typescript
export * from './workshop';
export * from './session';
export * from './api';
```

### `packages/types/src/workshop.ts`

```typescript
export type WorkshopStatus =
  | 'created'
  | 'provisioning'
  | 'running'
  | 'stopping'
  | 'stopped';

export interface Workshop {
  id: string;
  name: string;
  code: string;
  seats: number;
  status: WorkshopStatus;
  createdAt: string;
  vmName?: string;
  vmIp?: string;
  endpoint?: string;
}

export interface CreateWorkshopInput {
  name: string;
  seats: number;
  apiKey: string;
}

export interface WorkshopWithSecrets extends Workshop {
  apiKeySecretRef: string;
}
```

### `packages/types/src/session.ts`

```typescript
export type SessionStatus = 'active' | 'disconnected' | 'expired';

export interface Session {
  workshopId: string;
  containerId: string;
  seat: number;
  odehash: string;
}

export interface JoinInput {
  code: string;
  odehash?: string;
}

export interface JoinResult {
  token: string;
  endpoint: string;
  odehash: string;
  seat: number;
}

export interface TokenPayload {
  workshopId: string;
  containerId: string;
  seat: number;
  odehash: string;
  vmIp: string;
  exp: number;
}
```

### `packages/types/src/api.ts`

```typescript
import { Workshop, CreateWorkshopInput } from './workshop';
import { JoinInput, JoinResult } from './session';

// Request/Response types for API endpoints

export interface ApiResponse<T> {
  data?: T;
  error?: {
    code: string;
    message: string;
  };
}

export interface ListWorkshopsResponse {
  workshops: Workshop[];
}

export interface CreateWorkshopResponse {
  workshop: Workshop;
}

export interface StartWorkshopResponse {
  workshop: Workshop;
}

export interface StopWorkshopResponse {
  success: boolean;
}

export interface JoinWorkshopResponse extends JoinResult {}

// Terminal WebSocket messages
export type TerminalMessage =
  | { type: 'input'; data: string }
  | { type: 'output'; data: string }
  | { type: 'resize'; cols: number; rows: number };

// File API types
export interface FileInfo {
  name: string;
  path: string;
  isDirectory: boolean;
  size: number;
  modifiedAt: string;
}

export interface ListFilesResponse {
  files: FileInfo[];
}

export interface ReadFileResponse {
  content: string;
  encoding: 'utf-8' | 'base64';
}

export interface WriteFileInput {
  content: string;
  encoding?: 'utf-8' | 'base64';
}
```

---

## Portal API (apps/portal)

### `apps/portal/package.json`

```json
{
  "name": "@clarateach/portal",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@clarateach/types": "workspace:*",
    "@google-cloud/compute": "^4.0.0",
    "@google-cloud/secret-manager": "^5.0.0",
    "fastify": "^4.25.0",
    "@fastify/cors": "^8.5.0",
    "@fastify/jwt": "^8.0.0",
    "nanoid": "^5.0.0"
  },
  "devDependencies": {
    "tsx": "^4.7.0"
  }
}
```

### `apps/portal/src/index.ts`

```typescript
import Fastify from 'fastify';
import cors from '@fastify/cors';
import jwt from '@fastify/jwt';
import { workshopRoutes } from './routes/workshop';
import { sessionRoutes } from './routes/session';
import { healthRoutes } from './routes/health';
import { config } from './lib/config';

const app = Fastify({
  logger: true,
});

async function main() {
  // Register plugins
  await app.register(cors, {
    origin: config.corsOrigins,
    credentials: true,
  });

  await app.register(jwt, {
    secret: {
      private: config.jwtPrivateKey,
      public: config.jwtPublicKey,
    },
    sign: {
      algorithm: 'RS256',
      expiresIn: '24h',
    },
  });

  // Register routes
  await app.register(healthRoutes, { prefix: '/api' });
  await app.register(workshopRoutes, { prefix: '/api' });
  await app.register(sessionRoutes, { prefix: '/api' });

  // Start server
  const port = config.port;
  const host = config.host;

  await app.listen({ port, host });
  console.log(`Portal API running on ${host}:${port}`);
}

main().catch((err) => {
  console.error('Failed to start server:', err);
  process.exit(1);
});
```

### `apps/portal/src/lib/config.ts`

```typescript
import { readFileSync } from 'fs';

function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Missing required environment variable: ${name}`);
  }
  return value;
}

function loadKey(envVar: string, filePath?: string): string {
  if (process.env[envVar]) {
    return process.env[envVar]!;
  }
  if (filePath) {
    return readFileSync(filePath, 'utf-8');
  }
  throw new Error(`Missing ${envVar} or key file`);
}

export const config = {
  port: parseInt(process.env.PORT || '3000', 10),
  host: process.env.HOST || '0.0.0.0',

  gcpProject: requireEnv('GCP_PROJECT'),
  gcpZone: process.env.GCP_ZONE || 'us-central1-a',

  jwtPrivateKey: loadKey('JWT_PRIVATE_KEY', process.env.JWT_PRIVATE_KEY_FILE),
  jwtPublicKey: loadKey('JWT_PUBLIC_KEY', process.env.JWT_PUBLIC_KEY_FILE),

  corsOrigins: process.env.CORS_ORIGINS?.split(',') || ['http://localhost:5173'],

  workshopDomain: process.env.WORKSHOP_DOMAIN || 'clarateach.io',

  vmMachineType: process.env.VM_MACHINE_TYPE || 'e2-standard-8',
  vmImage: process.env.VM_IMAGE || 'ubuntu-os-cloud/ubuntu-2204-lts',
  vmDiskSize: parseInt(process.env.VM_DISK_SIZE || '100', 10),
  useSpotVms: process.env.USE_SPOT_VMS === 'true',
};
```

### `apps/portal/src/routes/workshop.ts`

```typescript
import { FastifyInstance, FastifyRequest } from 'fastify';
import { ComputeService } from '../services/compute';
import { SecretsService } from '../services/secrets';
import {
  CreateWorkshopInput,
  CreateWorkshopResponse,
  ListWorkshopsResponse,
  StartWorkshopResponse,
  StopWorkshopResponse,
  Workshop,
} from '@clarateach/types';
import { generateWorkshopCode, generateWorkshopId } from '../lib/generators';

export async function workshopRoutes(app: FastifyInstance) {
  const compute = new ComputeService();
  const secrets = new SecretsService();

  // List all workshops
  app.get<{
    Reply: ListWorkshopsResponse;
  }>('/workshops', async (request, reply) => {
    const workshops = await compute.listWorkshops();
    return { workshops };
  });

  // Get single workshop
  app.get<{
    Params: { id: string };
    Reply: { workshop: Workshop };
  }>('/workshops/:id', async (request, reply) => {
    const { id } = request.params;
    const workshop = await compute.getWorkshop(id);

    if (!workshop) {
      return reply.status(404).send({
        error: { code: 'NOT_FOUND', message: 'Workshop not found' }
      } as any);
    }

    return { workshop };
  });

  // Create workshop
  app.post<{
    Body: CreateWorkshopInput;
    Reply: CreateWorkshopResponse;
  }>('/workshops', async (request, reply) => {
    const { name, seats, apiKey } = request.body;

    // Validate
    if (seats < 1 || seats > 10) {
      return reply.status(400).send({
        error: { code: 'INVALID_SEATS', message: 'Seats must be between 1 and 10' }
      } as any);
    }

    // Generate identifiers
    const workshopId = generateWorkshopId();
    const code = generateWorkshopCode();

    // Store API key in Secret Manager
    const secretRef = await secrets.storeApiKey(workshopId, apiKey);

    // Create workshop (stores in GCP metadata or cache)
    const workshop = await compute.createWorkshop({
      id: workshopId,
      name,
      code,
      seats,
      apiKeySecretRef: secretRef,
    });

    return reply.status(201).send({ workshop });
  });

  // Start workshop (provision VM)
  app.post<{
    Params: { id: string };
    Reply: StartWorkshopResponse;
  }>('/workshops/:id/start', async (request, reply) => {
    const { id } = request.params;

    const workshop = await compute.getWorkshop(id);
    if (!workshop) {
      return reply.status(404).send({
        error: { code: 'NOT_FOUND', message: 'Workshop not found' }
      } as any);
    }

    if (workshop.status === 'running') {
      return reply.status(400).send({
        error: { code: 'ALREADY_RUNNING', message: 'Workshop is already running' }
      } as any);
    }

    // Provision VM
    const updatedWorkshop = await compute.provisionWorkspace(id);

    return reply.status(202).send({ workshop: updatedWorkshop });
  });

  // Stop workshop (destroy VM)
  app.post<{
    Params: { id: string };
    Reply: StopWorkshopResponse;
  }>('/workshops/:id/stop', async (request, reply) => {
    const { id } = request.params;

    const workshop = await compute.getWorkshop(id);
    if (!workshop) {
      return reply.status(404).send({
        error: { code: 'NOT_FOUND', message: 'Workshop not found' }
      } as any);
    }

    if (workshop.status !== 'running') {
      return reply.status(400).send({
        error: { code: 'NOT_RUNNING', message: 'Workshop is not running' }
      } as any);
    }

    // Destroy VM
    await compute.destroyWorkspace(id);

    // Delete API key secret
    await secrets.deleteApiKey(id);

    return { success: true };
  });
}
```

### `apps/portal/src/routes/session.ts`

```typescript
import { FastifyInstance } from 'fastify';
import { ComputeService } from '../services/compute';
import { JoinInput, JoinWorkshopResponse, TokenPayload } from '@clarateach/types';
import { generateOdehash } from '../lib/generators';

export async function sessionRoutes(app: FastifyInstance) {
  const compute = new ComputeService();

  // Join workshop
  app.post<{
    Body: JoinInput;
    Reply: JoinWorkshopResponse;
  }>('/join', async (request, reply) => {
    const { code, odehash: existingOdehash } = request.body;

    // Find workshop by code
    const workshop = await compute.findWorkshopByCode(code.toUpperCase());

    if (!workshop) {
      return reply.status(404).send({
        error: { code: 'NOT_FOUND', message: 'Workshop not found' }
      } as any);
    }

    if (workshop.status !== 'running') {
      return reply.status(400).send({
        error: { code: 'NOT_RUNNING', message: 'Workshop has not started yet' }
      } as any);
    }

    // Assign or retrieve seat
    let seat: number;
    let odehash: string;

    if (existingOdehash) {
      // Reconnecting learner
      const existingSeat = await compute.getSeatByOdehash(workshop.id, existingOdehash);
      if (existingSeat === null) {
        return reply.status(404).send({
          error: { code: 'SESSION_NOT_FOUND', message: 'Session not found. Try joining as new learner.' }
        } as any);
      }
      seat = existingSeat;
      odehash = existingOdehash;
    } else {
      // New learner
      odehash = generateOdehash();
      const assignedSeat = await compute.assignSeat(workshop.id, odehash);

      if (assignedSeat === null) {
        return reply.status(400).send({
          error: { code: 'NO_SEATS', message: 'Workshop is full' }
        } as any);
      }
      seat = assignedSeat;
    }

    // Create JWT token
    const tokenPayload: Omit<TokenPayload, 'exp'> = {
      workshopId: workshop.id,
      containerId: `c-${seat.toString().padStart(2, '0')}`,
      seat,
      odehash,
      vmIp: workshop.vmIp!,
    };

    const token = app.jwt.sign(tokenPayload);

    return {
      token,
      endpoint: workshop.endpoint!,
      odehash,
      seat,
    };
  });
}
```

### `apps/portal/src/services/compute.ts`

```typescript
import { InstancesClient, ZoneOperationsClient } from '@google-cloud/compute';
import { Workshop, WorkshopWithSecrets } from '@clarateach/types';
import { config } from '../lib/config';

interface CreateWorkshopParams {
  id: string;
  name: string;
  code: string;
  seats: number;
  apiKeySecretRef: string;
}

export class ComputeService {
  private instances: InstancesClient;
  private operations: ZoneOperationsClient;
  private project: string;
  private zone: string;

  constructor() {
    this.instances = new InstancesClient();
    this.operations = new ZoneOperationsClient();
    this.project = config.gcpProject;
    this.zone = config.gcpZone;
  }

  async listWorkshops(): Promise<Workshop[]> {
    const [instances] = await this.instances.list({
      project: this.project,
      zone: this.zone,
      filter: 'labels.type=clarateach-workshop',
    });

    return (instances || []).map(this.instanceToWorkshop);
  }

  async getWorkshop(id: string): Promise<Workshop | null> {
    try {
      const [instance] = await this.instances.get({
        project: this.project,
        zone: this.zone,
        instance: `clarateach-${id}`,
      });
      return this.instanceToWorkshop(instance);
    } catch (err: any) {
      if (err.code === 404) return null;
      throw err;
    }
  }

  async findWorkshopByCode(code: string): Promise<Workshop | null> {
    const [instances] = await this.instances.list({
      project: this.project,
      zone: this.zone,
      filter: `labels.type=clarateach-workshop AND labels.code=${code.toLowerCase()}`,
    });

    if (!instances || instances.length === 0) return null;
    return this.instanceToWorkshop(instances[0]);
  }

  async createWorkshop(params: CreateWorkshopParams): Promise<Workshop> {
    // For MVP, we store workshop data in instance metadata
    // The VM isn't created yet (just metadata placeholder)
    // In production, you might use Firestore here

    // Create a stopped placeholder instance or use cache
    // For now, we'll create the actual VM in provisionWorkspace

    return {
      id: params.id,
      name: params.name,
      code: params.code,
      seats: params.seats,
      status: 'created',
      createdAt: new Date().toISOString(),
    };
  }

  async provisionWorkspace(workshopId: string): Promise<Workshop> {
    const vmName = `clarateach-${workshopId}`;

    const startupScript = this.generateStartupScript(workshopId);

    const [operation] = await this.instances.insert({
      project: this.project,
      zone: this.zone,
      instanceResource: {
        name: vmName,
        machineType: `zones/${this.zone}/machineTypes/${config.vmMachineType}`,
        labels: {
          type: 'clarateach-workshop',
          'workshop-id': workshopId,
        },
        disks: [{
          boot: true,
          autoDelete: true,
          initializeParams: {
            sourceImage: `projects/${config.vmImage}`,
            diskSizeGb: config.vmDiskSize.toString(),
            diskType: `zones/${this.zone}/diskTypes/pd-ssd`,
          },
        }],
        networkInterfaces: [{
          network: 'global/networks/default',
          accessConfigs: [{
            name: 'External NAT',
            type: 'ONE_TO_ONE_NAT',
          }],
        }],
        metadata: {
          items: [
            { key: 'startup-script', value: startupScript },
            { key: 'workshop-id', value: workshopId },
            { key: 'seats-map', value: '{}' },
          ],
        },
        serviceAccounts: [{
          scopes: ['https://www.googleapis.com/auth/cloud-platform'],
        }],
        scheduling: config.useSpotVms ? {
          preemptible: true,
          automaticRestart: false,
        } : undefined,
      },
    });

    // Wait for operation to complete
    await this.waitForOperation(operation.latestResponse.name!);

    // Get the created instance
    const [instance] = await this.instances.get({
      project: this.project,
      zone: this.zone,
      instance: vmName,
    });

    return this.instanceToWorkshop(instance);
  }

  async destroyWorkspace(workshopId: string): Promise<void> {
    const vmName = `clarateach-${workshopId}`;

    const [operation] = await this.instances.delete({
      project: this.project,
      zone: this.zone,
      instance: vmName,
    });

    await this.waitForOperation(operation.latestResponse.name!);
  }

  async assignSeat(workshopId: string, odehash: string): Promise<number | null> {
    const vmName = `clarateach-${workshopId}`;

    // Get current metadata
    const [instance] = await this.instances.get({
      project: this.project,
      zone: this.zone,
      instance: vmName,
    });

    const seatsMapItem = instance.metadata?.items?.find(i => i.key === 'seats-map');
    const seatsMap: Record<string, number> = seatsMapItem?.value
      ? JSON.parse(seatsMapItem.value)
      : {};

    const seatsItem = instance.labels?.seats;
    const maxSeats = seatsItem ? parseInt(seatsItem, 10) : 10;

    // Find next available seat
    const usedSeats = new Set(Object.values(seatsMap));
    let seat = 1;
    while (usedSeats.has(seat) && seat <= maxSeats) {
      seat++;
    }

    if (seat > maxSeats) {
      return null; // No seats available
    }

    // Assign seat
    seatsMap[odehash] = seat;

    // Update metadata
    await this.instances.setMetadata({
      project: this.project,
      zone: this.zone,
      instance: vmName,
      metadataResource: {
        fingerprint: instance.metadata?.fingerprint,
        items: [
          ...(instance.metadata?.items?.filter(i => i.key !== 'seats-map') || []),
          { key: 'seats-map', value: JSON.stringify(seatsMap) },
        ],
      },
    });

    return seat;
  }

  async getSeatByOdehash(workshopId: string, odehash: string): Promise<number | null> {
    const vmName = `clarateach-${workshopId}`;

    const [instance] = await this.instances.get({
      project: this.project,
      zone: this.zone,
      instance: vmName,
    });

    const seatsMapItem = instance.metadata?.items?.find(i => i.key === 'seats-map');
    const seatsMap: Record<string, number> = seatsMapItem?.value
      ? JSON.parse(seatsMapItem.value)
      : {};

    return seatsMap[odehash] ?? null;
  }

  private async waitForOperation(operationName: string): Promise<void> {
    let status = 'RUNNING';
    while (status === 'RUNNING') {
      await new Promise(resolve => setTimeout(resolve, 2000));
      const [operation] = await this.operations.get({
        project: this.project,
        zone: this.zone,
        operation: operationName,
      });
      status = operation.status || 'DONE';
    }
  }

  private instanceToWorkshop(instance: any): Workshop {
    const vmIp = instance.networkInterfaces?.[0]?.accessConfigs?.[0]?.natIP;
    const status = instance.status === 'RUNNING' ? 'running' : 'stopped';

    return {
      id: instance.labels?.['workshop-id'] || instance.name?.replace('clarateach-', ''),
      name: instance.metadata?.items?.find((i: any) => i.key === 'workshop-name')?.value || 'Workshop',
      code: instance.labels?.code?.toUpperCase() || '',
      seats: parseInt(instance.labels?.seats || '10', 10),
      status,
      createdAt: instance.creationTimestamp,
      vmName: instance.name,
      vmIp,
      endpoint: vmIp ? `https://${vmIp}` : undefined,
    };
  }

  private generateStartupScript(workshopId: string): string {
    return `#!/bin/bash
set -e

# Install Docker
curl -fsSL https://get.docker.com | sh

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Create workspace directory
mkdir -p /opt/clarateach
cd /opt/clarateach

# Download docker-compose.yml (from your artifact registry or storage)
# For MVP, embed it:
cat > docker-compose.yml << 'EOF'
${this.getDockerComposeContent()}
EOF

# Start services
docker-compose up -d

echo "ClaraTeach workspace ready"
`;
  }

  private getDockerComposeContent(): string {
    return `
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy-data:/data

  # Learner containers would be dynamically created
  # This is a simplified example

volumes:
  caddy-data:
`;
  }
}
```

### `apps/portal/src/lib/generators.ts`

```typescript
import { nanoid, customAlphabet } from 'nanoid';

// Workshop ID: ws-abc123
export function generateWorkshopId(): string {
  return `ws-${nanoid(6)}`;
}

// Workshop code: CLAUDE-2024 (human-readable)
const codeAlphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'; // No I, O, 0, 1
const generateCode = customAlphabet(codeAlphabet, 4);

export function generateWorkshopCode(): string {
  return `CLAUDE-${generateCode()}`;
}

// Odehash: x7k2m (short reconnect code)
const odehashAlphabet = 'abcdefghjkmnpqrstuvwxyz23456789';
const generateOde = customAlphabet(odehashAlphabet, 5);

export function generateOdehash(): string {
  return generateOde();
}
```

---

## Web Frontend (apps/web)

### `apps/web/package.json`

```json
{
  "name": "@clarateach/web",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@clarateach/types": "workspace:*",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.21.0",
    "@monaco-editor/react": "^4.6.0",
    "xterm": "^5.3.0",
    "xterm-addon-fit": "^0.8.0",
    "xterm-addon-web-links": "^0.9.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.2.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0",
    "tailwindcss": "^3.4.0",
    "vite": "^5.0.0"
  }
}
```

### `apps/web/src/App.tsx`

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Join } from './pages/Join';
import { Workspace } from './pages/Workspace';
import { Admin } from './pages/Admin';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Join />} />
        <Route path="/join" element={<Join />} />
        <Route path="/workspace" element={<Workspace />} />
        <Route path="/admin" element={<Admin />} />
      </Routes>
    </BrowserRouter>
  );
}
```

### `apps/web/src/pages/Workspace.tsx`

```tsx
import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Terminal } from '../components/Terminal';
import { CodeEditor } from '../components/CodeEditor';
import { Browser } from '../components/Browser';
import { ResizablePanelGroup, ResizablePanel, ResizableHandle } from '../components/ui/resizable';
import { decodeToken } from '../lib/token';
import type { TokenPayload } from '@clarateach/types';

export function Workspace() {
  const [params] = useSearchParams();
  const [session, setSession] = useState<TokenPayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const token = params.get('token');
    if (!token) {
      setError('No session token provided');
      return;
    }

    try {
      const payload = decodeToken(token);
      setSession(payload);
    } catch (err) {
      setError('Invalid session token');
    }
  }, [params]);

  if (error) {
    return (
      <div className="h-screen w-screen bg-[#1e1e1e] flex items-center justify-center">
        <div className="text-red-400">{error}</div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="h-screen w-screen bg-[#1e1e1e] flex items-center justify-center">
        <div className="text-gray-400">Loading...</div>
      </div>
    );
  }

  const wsBase = `wss://${session.vmIp}/vm/${session.seat.toString().padStart(2, '0')}`;

  return (
    <div className="h-screen w-screen bg-[#1e1e1e] overflow-hidden">
      {/* Header */}
      <div className="bg-[#323233] border-b border-[#3e3e3e] px-4 py-3 flex justify-between items-center">
        <h1 className="text-white font-medium">ClaraTeach</h1>
        <div className="text-gray-400 text-sm">
          Reconnect code: <code className="text-white bg-[#1e1e1e] px-2 py-1 rounded">{session.odehash}</code>
        </div>
      </div>

      {/* Main Content */}
      <div className="h-[calc(100vh-57px)]">
        <ResizablePanelGroup direction="horizontal">
          {/* Left Section - Code Editor and Terminal */}
          <ResizablePanel defaultSize={60} minSize={30}>
            <ResizablePanelGroup direction="vertical">
              {/* Code Editor */}
              <ResizablePanel defaultSize={60} minSize={30}>
                <CodeEditor
                  endpoint={`https://${session.vmIp}/vm/${session.seat.toString().padStart(2, '0')}/files`}
                />
              </ResizablePanel>

              <ResizableHandle withHandle />

              {/* Terminal */}
              <ResizablePanel defaultSize={40} minSize={20}>
                <Terminal wsUrl={`${wsBase}/terminal`} />
              </ResizablePanel>
            </ResizablePanelGroup>
          </ResizablePanel>

          <ResizableHandle withHandle />

          {/* Right Section - Browser (Full Height) */}
          <ResizablePanel defaultSize={40} minSize={25}>
            <Browser wsUrl={`${wsBase}/browser`} />
          </ResizablePanel>
        </ResizablePanelGroup>
      </div>
    </div>
  );
}
```

### `apps/web/src/components/Terminal.tsx`

```tsx
import { useEffect, useRef } from 'react';
import { Terminal as XTerm } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import 'xterm/css/xterm.css';

interface TerminalProps {
  wsUrl: string;
}

export function Terminal({ wsUrl }: TerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<XTerm | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    // Create terminal
    const terminal = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
        cursor: '#d4d4d4',
      },
    });

    const fitAddon = new FitAddon();
    const webLinksAddon = new WebLinksAddon();

    terminal.loadAddon(fitAddon);
    terminal.loadAddon(webLinksAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();

    terminalRef.current = terminal;

    // Connect WebSocket
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('Terminal connected');
      // Send initial size
      ws.send(JSON.stringify({
        type: 'resize',
        cols: terminal.cols,
        rows: terminal.rows,
      }));
    };

    ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      if (message.type === 'output') {
        terminal.write(message.data);
      }
    };

    ws.onerror = (error) => {
      console.error('Terminal WebSocket error:', error);
      terminal.write('\r\n\x1b[31mConnection error\x1b[0m\r\n');
    };

    ws.onclose = () => {
      console.log('Terminal disconnected');
      terminal.write('\r\n\x1b[33mDisconnected\x1b[0m\r\n');
    };

    // Handle input
    terminal.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
      fitAddon.fit();
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'resize',
          cols: terminal.cols,
          rows: terminal.rows,
        }));
      }
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      ws.close();
      terminal.dispose();
    };
  }, [wsUrl]);

  return (
    <div
      ref={containerRef}
      className="h-full w-full bg-[#1e1e1e] p-2"
    />
  );
}
```

### `apps/web/src/components/CodeEditor.tsx`

```tsx
import { useState, useEffect } from 'react';
import Editor from '@monaco-editor/react';
import type { FileInfo } from '@clarateach/types';

interface CodeEditorProps {
  endpoint: string;
}

export function CodeEditor({ endpoint }: CodeEditorProps) {
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [currentFile, setCurrentFile] = useState<string | null>(null);
  const [content, setContent] = useState<string>('');
  const [modified, setModified] = useState(false);

  // Load file tree
  useEffect(() => {
    fetch(`${endpoint}?path=/workspace`)
      .then(res => res.json())
      .then(data => setFiles(data.files))
      .catch(console.error);
  }, [endpoint]);

  // Load file content
  const openFile = async (path: string) => {
    try {
      const res = await fetch(`${endpoint}${path}`);
      const data = await res.json();
      setCurrentFile(path);
      setContent(data.content);
      setModified(false);
    } catch (err) {
      console.error('Failed to load file:', err);
    }
  };

  // Save file
  const saveFile = async () => {
    if (!currentFile) return;

    try {
      await fetch(`${endpoint}${currentFile}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content }),
      });
      setModified(false);
    } catch (err) {
      console.error('Failed to save file:', err);
    }
  };

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        saveFile();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [currentFile, content]);

  return (
    <div className="h-full flex">
      {/* File tree */}
      <div className="w-48 bg-[#252526] border-r border-[#3e3e3e] overflow-auto">
        <div className="p-2 text-xs text-gray-400 uppercase">Files</div>
        {files.map(file => (
          <div
            key={file.path}
            className={`px-2 py-1 text-sm cursor-pointer hover:bg-[#2a2d2e] ${
              currentFile === file.path ? 'bg-[#37373d]' : ''
            }`}
            onClick={() => !file.isDirectory && openFile(file.path)}
          >
            <span className="text-gray-300">
              {file.isDirectory ? '📁 ' : '📄 '}
              {file.name}
            </span>
          </div>
        ))}
      </div>

      {/* Editor */}
      <div className="flex-1">
        {currentFile ? (
          <>
            <div className="bg-[#252526] px-3 py-1 text-sm text-gray-400 border-b border-[#3e3e3e]">
              {currentFile} {modified && '•'}
            </div>
            <Editor
              height="calc(100% - 28px)"
              defaultLanguage="python"
              theme="vs-dark"
              value={content}
              onChange={(value) => {
                setContent(value || '');
                setModified(true);
              }}
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                lineNumbers: 'on',
                scrollBeyondLastLine: false,
              }}
            />
          </>
        ) : (
          <div className="h-full flex items-center justify-center text-gray-500">
            Select a file to edit
          </div>
        )}
      </div>
    </div>
  );
}
```

### `apps/web/src/components/Browser.tsx`

```tsx
import { useEffect, useRef, useState } from 'react';

interface BrowserProps {
  wsUrl: string;
}

export function Browser({ wsUrl }: BrowserProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'error'>('connecting');

  useEffect(() => {
    // neko uses WebRTC for video streaming
    // This is a simplified example - actual neko integration requires their client library

    const ws = new WebSocket(wsUrl);
    let peerConnection: RTCPeerConnection | null = null;

    ws.onopen = () => {
      console.log('Browser WebSocket connected');
      // Send connect message to neko
      ws.send(JSON.stringify({ event: 'connect' }));
    };

    ws.onmessage = async (event) => {
      const message = JSON.parse(event.data);

      switch (message.event) {
        case 'offer':
          // Handle WebRTC offer from neko
          peerConnection = new RTCPeerConnection({
            iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
          });

          peerConnection.ontrack = (e) => {
            if (videoRef.current && e.streams[0]) {
              videoRef.current.srcObject = e.streams[0];
              setStatus('connected');
            }
          };

          peerConnection.onicecandidate = (e) => {
            if (e.candidate) {
              ws.send(JSON.stringify({
                event: 'candidate',
                data: e.candidate,
              }));
            }
          };

          await peerConnection.setRemoteDescription(message.data);
          const answer = await peerConnection.createAnswer();
          await peerConnection.setLocalDescription(answer);

          ws.send(JSON.stringify({
            event: 'answer',
            data: answer,
          }));
          break;

        case 'candidate':
          if (peerConnection) {
            await peerConnection.addIceCandidate(message.data);
          }
          break;
      }
    };

    ws.onerror = () => {
      setStatus('error');
    };

    return () => {
      ws.close();
      peerConnection?.close();
    };
  }, [wsUrl]);

  return (
    <div className="h-full bg-[#1e1e1e] flex flex-col">
      <div className="bg-[#252526] px-3 py-1 text-sm text-gray-400 border-b border-[#3e3e3e] flex items-center gap-2">
        <span className={`w-2 h-2 rounded-full ${
          status === 'connected' ? 'bg-green-500' :
          status === 'error' ? 'bg-red-500' : 'bg-yellow-500'
        }`} />
        Browser Preview (view only)
      </div>
      <div className="flex-1 relative">
        {status === 'connecting' && (
          <div className="absolute inset-0 flex items-center justify-center text-gray-500">
            Connecting to browser...
          </div>
        )}
        {status === 'error' && (
          <div className="absolute inset-0 flex items-center justify-center text-red-400">
            Failed to connect to browser preview
          </div>
        )}
        <video
          ref={videoRef}
          autoPlay
          playsInline
          muted
          className="w-full h-full object-contain"
        />
      </div>
    </div>
  );
}
```

---

## Workspace Server (apps/workspace)

Runs inside each learner container.

### `apps/workspace/package.json`

```json
{
  "name": "@clarateach/workspace",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js"
  },
  "dependencies": {
    "@clarateach/types": "workspace:*",
    "fastify": "^4.25.0",
    "@fastify/websocket": "^8.3.0",
    "node-pty": "^1.0.0"
  },
  "devDependencies": {
    "tsx": "^4.7.0"
  }
}
```

### `apps/workspace/src/index.ts`

```typescript
import Fastify from 'fastify';
import websocket from '@fastify/websocket';
import { terminalRoutes } from './routes/terminal';
import { filesRoutes } from './routes/files';

const app = Fastify({ logger: true });

async function main() {
  await app.register(websocket);
  await app.register(terminalRoutes, { prefix: '/terminal' });
  await app.register(filesRoutes, { prefix: '/files' });

  const port = parseInt(process.env.PORT || '3001', 10);
  await app.listen({ port, host: '0.0.0.0' });
  console.log(`Workspace server running on port ${port}`);
}

main().catch(console.error);
```

### `apps/workspace/src/routes/terminal.ts`

```typescript
import { FastifyInstance } from 'fastify';
import * as pty from 'node-pty';
import type { TerminalMessage } from '@clarateach/types';

export async function terminalRoutes(app: FastifyInstance) {
  app.get('/', { websocket: true }, (connection, req) => {
    // Spawn PTY attached to tmux
    const shell = pty.spawn('tmux', ['attach-session', '-t', 'main'], {
      name: 'xterm-256color',
      cols: 80,
      rows: 24,
      cwd: process.env.WORKSPACE_DIR || '/workspace',
      env: {
        ...process.env,
        TERM: 'xterm-256color',
      },
    });

    // Handle PTY output
    shell.onData((data) => {
      connection.socket.send(JSON.stringify({
        type: 'output',
        data,
      }));
    });

    // Handle client messages
    connection.socket.on('message', (raw) => {
      try {
        const message: TerminalMessage = JSON.parse(raw.toString());

        switch (message.type) {
          case 'input':
            shell.write(message.data);
            break;
          case 'resize':
            shell.resize(message.cols, message.rows);
            break;
        }
      } catch (err) {
        console.error('Invalid terminal message:', err);
      }
    });

    // Handle disconnect
    connection.socket.on('close', () => {
      // Don't kill shell - it's in tmux, will persist
      console.log('Client disconnected from terminal');
    });
  });
}
```

### `apps/workspace/src/routes/files.ts`

```typescript
import { FastifyInstance } from 'fastify';
import { readdir, readFile, writeFile, stat } from 'fs/promises';
import { join, relative } from 'path';
import type { FileInfo, ListFilesResponse, ReadFileResponse, WriteFileInput } from '@clarateach/types';

const WORKSPACE_DIR = process.env.WORKSPACE_DIR || '/workspace';

export async function filesRoutes(app: FastifyInstance) {
  // List files
  app.get<{
    Querystring: { path?: string };
    Reply: ListFilesResponse;
  }>('/', async (request, reply) => {
    const requestedPath = request.query.path || '/workspace';
    const fullPath = join(WORKSPACE_DIR, relative('/workspace', requestedPath));

    // Security: ensure path is within workspace
    if (!fullPath.startsWith(WORKSPACE_DIR)) {
      return reply.status(403).send({ error: 'Access denied' } as any);
    }

    const entries = await readdir(fullPath, { withFileTypes: true });
    const files: FileInfo[] = await Promise.all(
      entries.map(async (entry) => {
        const entryPath = join(fullPath, entry.name);
        const stats = await stat(entryPath);
        return {
          name: entry.name,
          path: join(requestedPath, entry.name),
          isDirectory: entry.isDirectory(),
          size: stats.size,
          modifiedAt: stats.mtime.toISOString(),
        };
      })
    );

    return { files };
  });

  // Read file
  app.get<{
    Params: { '*': string };
    Reply: ReadFileResponse;
  }>('/*', async (request, reply) => {
    const requestedPath = request.params['*'];
    const fullPath = join(WORKSPACE_DIR, requestedPath);

    if (!fullPath.startsWith(WORKSPACE_DIR)) {
      return reply.status(403).send({ error: 'Access denied' } as any);
    }

    try {
      const content = await readFile(fullPath, 'utf-8');
      return { content, encoding: 'utf-8' };
    } catch (err: any) {
      if (err.code === 'ENOENT') {
        return reply.status(404).send({ error: 'File not found' } as any);
      }
      throw err;
    }
  });

  // Write file
  app.put<{
    Params: { '*': string };
    Body: WriteFileInput;
  }>('/*', async (request, reply) => {
    const requestedPath = request.params['*'];
    const fullPath = join(WORKSPACE_DIR, requestedPath);

    if (!fullPath.startsWith(WORKSPACE_DIR)) {
      return reply.status(403).send({ error: 'Access denied' } as any);
    }

    const { content } = request.body;
    await writeFile(fullPath, content, 'utf-8');

    return { success: true };
  });
}
```

---

## CLI Tool (cli/)

### `cli/package.json`

```json
{
  "name": "@clarateach/cli",
  "version": "0.1.0",
  "bin": {
    "clarateach": "./dist/index.js"
  },
  "scripts": {
    "build": "tsc",
    "dev": "tsx src/index.ts"
  },
  "dependencies": {
    "commander": "^11.1.0",
    "inquirer": "^9.2.0",
    "chalk": "^5.3.0",
    "ora": "^7.0.0"
  }
}
```

### `cli/src/index.ts`

```typescript
#!/usr/bin/env node

import { Command } from 'commander';
import { workshopCommands } from './commands/workshop';

const program = new Command();

program
  .name('clarateach')
  .description('ClaraTeach CLI for instructors')
  .version('0.1.0');

workshopCommands(program);

program.parse();
```

### `cli/src/commands/workshop.ts`

```typescript
import { Command } from 'commander';
import inquirer from 'inquirer';
import chalk from 'chalk';
import ora from 'ora';
import { api } from '../lib/api';

export function workshopCommands(program: Command) {
  const workshop = program.command('workshop');

  workshop
    .command('create')
    .description('Create a new workshop')
    .option('-n, --name <name>', 'Workshop name')
    .option('-s, --seats <seats>', 'Number of seats', '10')
    .option('-k, --api-key <key>', 'Claude API key')
    .action(async (opts) => {
      let { name, seats, apiKey } = opts;

      // Interactive prompts for missing options
      if (!name || !apiKey) {
        const answers = await inquirer.prompt([
          {
            type: 'input',
            name: 'name',
            message: 'Workshop name:',
            when: !name,
          },
          {
            type: 'password',
            name: 'apiKey',
            message: 'Claude API key:',
            when: !apiKey,
          },
        ]);
        name = name || answers.name;
        apiKey = apiKey || answers.apiKey;
      }

      const spinner = ora('Creating workshop...').start();

      try {
        const result = await api.createWorkshop({
          name,
          seats: parseInt(seats, 10),
          apiKey,
        });

        spinner.succeed('Workshop created!');
        console.log('');
        console.log(`  ${chalk.bold('Workshop ID:')} ${result.workshop.id}`);
        console.log(`  ${chalk.bold('Code:')} ${chalk.green(result.workshop.code)}`);
        console.log(`  ${chalk.bold('Seats:')} ${result.workshop.seats}`);
        console.log('');
        console.log(chalk.dim('Run `clarateach workshop start ' + result.workshop.id + '` to provision.'));
      } catch (err: any) {
        spinner.fail('Failed to create workshop');
        console.error(chalk.red(err.message));
        process.exit(1);
      }
    });

  workshop
    .command('start <id>')
    .description('Start a workshop (provision VM)')
    .action(async (id) => {
      const spinner = ora('Provisioning workspace...').start();

      try {
        const result = await api.startWorkshop(id);
        spinner.succeed('Workshop running!');
        console.log('');
        console.log(`  ${chalk.bold('Endpoint:')} ${chalk.cyan(result.workshop.endpoint)}`);
        console.log(`  ${chalk.bold('Code:')} ${chalk.green(result.workshop.code)}`);
        console.log('');
        console.log('Share the code with learners to join.');
      } catch (err: any) {
        spinner.fail('Failed to start workshop');
        console.error(chalk.red(err.message));
        process.exit(1);
      }
    });

  workshop
    .command('stop <id>')
    .description('Stop a workshop (destroy VM)')
    .action(async (id) => {
      const { confirm } = await inquirer.prompt([{
        type: 'confirm',
        name: 'confirm',
        message: 'This will destroy all learner environments. Continue?',
        default: false,
      }]);

      if (!confirm) {
        console.log('Cancelled.');
        return;
      }

      const spinner = ora('Stopping workshop...').start();

      try {
        await api.stopWorkshop(id);
        spinner.succeed('Workshop stopped');
      } catch (err: any) {
        spinner.fail('Failed to stop workshop');
        console.error(chalk.red(err.message));
        process.exit(1);
      }
    });

  workshop
    .command('list')
    .description('List all workshops')
    .action(async () => {
      const spinner = ora('Loading workshops...').start();

      try {
        const result = await api.listWorkshops();
        spinner.stop();

        if (result.workshops.length === 0) {
          console.log(chalk.dim('No workshops found.'));
          return;
        }

        console.log('');
        console.log(chalk.bold('Workshops:'));
        console.log('');

        for (const w of result.workshops) {
          const statusColor = w.status === 'running' ? chalk.green : chalk.gray;
          console.log(`  ${chalk.bold(w.id)} - ${w.name}`);
          console.log(`    Code: ${w.code} | Seats: ${w.seats} | Status: ${statusColor(w.status)}`);
          if (w.endpoint) {
            console.log(`    Endpoint: ${chalk.cyan(w.endpoint)}`);
          }
          console.log('');
        }
      } catch (err: any) {
        spinner.fail('Failed to list workshops');
        console.error(chalk.red(err.message));
        process.exit(1);
      }
    });
}
```

### `cli/src/lib/api.ts`

```typescript
import {
  CreateWorkshopInput,
  CreateWorkshopResponse,
  ListWorkshopsResponse,
  StartWorkshopResponse,
} from '@clarateach/types';

const API_URL = process.env.CLARATEACH_API_URL || 'https://api.clarateach.io';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!res.ok) {
    const error = await res.json().catch(() => ({}));
    throw new Error(error.message || `Request failed: ${res.status}`);
  }

  return res.json();
}

export const api = {
  async listWorkshops(): Promise<ListWorkshopsResponse> {
    return request('/api/workshops');
  },

  async createWorkshop(input: CreateWorkshopInput): Promise<CreateWorkshopResponse> {
    return request('/api/workshops', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  },

  async startWorkshop(id: string): Promise<StartWorkshopResponse> {
    return request(`/api/workshops/${id}/start`, {
      method: 'POST',
    });
  },

  async stopWorkshop(id: string): Promise<void> {
    await request(`/api/workshops/${id}/stop`, {
      method: 'POST',
    });
  },
};
```

---

## Docker Configuration

### `containers/workspace/Dockerfile`

```dockerfile
FROM ubuntu:22.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    tmux \
    python3 \
    python3-pip \
    nodejs \
    npm \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Claude CLI
RUN npm install -g @anthropic-ai/claude-code

# Install workspace server
WORKDIR /opt/workspace-server
COPY apps/workspace/dist ./
COPY apps/workspace/package.json ./
RUN npm install --production

# Create learner user
RUN useradd -m -s /bin/bash learner

# Setup tmux
COPY containers/workspace/config/tmux.conf /home/learner/.tmux.conf
RUN chown learner:learner /home/learner/.tmux.conf

# Setup workspace directory
RUN mkdir -p /workspace && chown learner:learner /workspace

# Entrypoint
COPY containers/workspace/scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER learner
WORKDIR /workspace

ENV WORKSPACE_DIR=/workspace

EXPOSE 3001 3002

ENTRYPOINT ["/entrypoint.sh"]
```

### `containers/workspace/scripts/entrypoint.sh`

```bash
#!/bin/bash
set -e

# Start tmux session if not exists
tmux has-session -t main 2>/dev/null || tmux new-session -d -s main

# Configure Claude CLI with API key
if [ -n "$CLAUDE_API_KEY" ]; then
  mkdir -p ~/.config/claude
  echo "{\"apiKey\": \"$CLAUDE_API_KEY\"}" > ~/.config/claude/config.json
fi

# Start workspace server
cd /opt/workspace-server
exec node index.js
```

### `containers/workspace/config/tmux.conf`

```
# Better colors
set -g default-terminal "xterm-256color"

# Mouse support
set -g mouse on

# Increase history
set -g history-limit 10000

# Start windows and panes at 1, not 0
set -g base-index 1
setw -g pane-base-index 1

# Status bar
set -g status-style bg=colour235,fg=colour136
set -g status-left '#[fg=colour46]#S '
set -g status-right '#[fg=colour136]%H:%M'
```

---

## Local Development

### `docker-compose.yml`

```yaml
version: '3.8'

services:
  portal:
    build:
      context: .
      dockerfile: apps/portal/Dockerfile
    ports:
      - "3000:3000"
    environment:
      - NODE_ENV=development
      - GCP_PROJECT=${GCP_PROJECT}
      - JWT_PRIVATE_KEY_FILE=/secrets/jwt.key
      - JWT_PUBLIC_KEY_FILE=/secrets/jwt.key.pub
    volumes:
      - ./apps/portal/src:/app/src
      - ./secrets:/secrets:ro

  web:
    build:
      context: .
      dockerfile: apps/web/Dockerfile
    ports:
      - "5173:5173"
    volumes:
      - ./apps/web/src:/app/src
    environment:
      - VITE_API_URL=http://localhost:3000

  workspace:
    build:
      context: .
      dockerfile: containers/workspace/Dockerfile
    ports:
      - "3001:3001"
      - "3002:3002"
    environment:
      - CLAUDE_API_KEY=${CLAUDE_API_KEY}
    volumes:
      - workspace-data:/workspace

volumes:
  workspace-data:
```

---

## Summary

This TypeScript implementation provides:

1. **Monorepo structure** with pnpm workspaces and Turborepo
2. **Portal API** using Fastify with GCP integration
3. **React frontend** with xterm.js and Monaco Editor
4. **Workspace server** with PTY and file system access
5. **CLI tool** for instructors
6. **Docker containers** for learner environments
7. **Shared types** package for type safety across all apps
