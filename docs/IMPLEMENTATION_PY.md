# ClaraTeach - Python Implementation

This document describes the Python implementation of ClaraTeach.

---

## Project Structure

```
clarateach/
├── pyproject.toml                  # Project configuration (using uv/poetry)
├── README.md
│
├── src/
│   └── clarateach/
│       ├── __init__.py
│       ├── portal/                 # Admin API (Cloud Run)
│       │   ├── __init__.py
│       │   ├── main.py
│       │   ├── routes/
│       │   ├── services/
│       │   └── config.py
│       │
│       ├── workspace/              # Container workspace server
│       │   ├── __init__.py
│       │   ├── main.py
│       │   ├── routes/
│       │   └── terminal.py
│       │
│       ├── cli/                    # Instructor CLI
│       │   ├── __init__.py
│       │   ├── main.py
│       │   └── commands/
│       │
│       └── shared/                 # Shared types and utilities
│           ├── __init__.py
│           ├── models.py
│           └── utils.py
│
├── web/                            # React Frontend (same as TS version)
│   ├── package.json
│   └── src/
│
├── containers/
│   ├── workspace/
│   │   ├── Dockerfile
│   │   └── scripts/
│   └── neko/
│
├── infrastructure/                 # Terraform
│
└── tests/
    ├── portal/
    ├── workspace/
    └── cli/
```

---

## Package Configuration

### `pyproject.toml`

```toml
[project]
name = "clarateach"
version = "0.1.0"
description = "Cloud-based learning platform for Claude CLI workshops"
readme = "README.md"
requires-python = ">=3.11"
dependencies = [
    "fastapi>=0.109.0",
    "uvicorn[standard]>=0.27.0",
    "websockets>=12.0",
    "pydantic>=2.5.0",
    "pydantic-settings>=2.1.0",
    "google-cloud-compute>=1.15.0",
    "google-cloud-secret-manager>=2.17.0",
    "python-jose[cryptography]>=3.3.0",
    "httpx>=0.26.0",
    "typer>=0.9.0",
    "rich>=13.7.0",
    "ptyprocess>=0.7.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=7.4.0",
    "pytest-asyncio>=0.23.0",
    "pytest-cov>=4.1.0",
    "ruff>=0.1.0",
    "mypy>=1.8.0",
    "black>=24.1.0",
]

[project.scripts]
clarateach = "clarateach.cli.main:app"
clarateach-portal = "clarateach.portal.main:run"
clarateach-workspace = "clarateach.workspace.main:run"

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["src/clarateach"]

[tool.ruff]
line-length = 100
target-version = "py311"

[tool.ruff.lint]
select = ["E", "F", "I", "N", "W", "UP"]

[tool.mypy]
python_version = "3.11"
strict = true

[tool.pytest.ini_options]
asyncio_mode = "auto"
testpaths = ["tests"]
```

---

## Shared Models

### `src/clarateach/shared/models.py`

```python
from datetime import datetime
from enum import Enum
from typing import Optional

from pydantic import BaseModel, Field


class WorkshopStatus(str, Enum):
    CREATED = "created"
    PROVISIONING = "provisioning"
    RUNNING = "running"
    STOPPING = "stopping"
    STOPPED = "stopped"


class SessionStatus(str, Enum):
    ACTIVE = "active"
    DISCONNECTED = "disconnected"
    EXPIRED = "expired"


# Workshop Models


class Workshop(BaseModel):
    id: str
    name: str
    code: str
    seats: int
    status: WorkshopStatus
    created_at: datetime
    vm_name: Optional[str] = None
    vm_ip: Optional[str] = None
    endpoint: Optional[str] = None


class CreateWorkshopRequest(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    seats: int = Field(..., ge=1, le=10)
    api_key: str = Field(..., min_length=10)


class CreateWorkshopResponse(BaseModel):
    workshop: Workshop


class ListWorkshopsResponse(BaseModel):
    workshops: list[Workshop]


class StartWorkshopResponse(BaseModel):
    workshop: Workshop


class StopWorkshopResponse(BaseModel):
    success: bool


# Session Models


class Session(BaseModel):
    workshop_id: str
    container_id: str
    seat: int
    odehash: str


class JoinRequest(BaseModel):
    code: str = Field(..., min_length=4, max_length=20)
    odehash: Optional[str] = None


class JoinResponse(BaseModel):
    token: str
    endpoint: str
    odehash: str
    seat: int


class TokenPayload(BaseModel):
    workshop_id: str
    container_id: str
    seat: int
    odehash: str
    vm_ip: str
    exp: int


# Terminal WebSocket Messages


class TerminalInputMessage(BaseModel):
    type: str = "input"
    data: str


class TerminalOutputMessage(BaseModel):
    type: str = "output"
    data: str


class TerminalResizeMessage(BaseModel):
    type: str = "resize"
    cols: int
    rows: int


# File API Models


class FileInfo(BaseModel):
    name: str
    path: str
    is_directory: bool
    size: int
    modified_at: datetime


class ListFilesResponse(BaseModel):
    files: list[FileInfo]


class ReadFileResponse(BaseModel):
    content: str
    encoding: str = "utf-8"


class WriteFileRequest(BaseModel):
    content: str
    encoding: str = "utf-8"


# API Error


class APIError(BaseModel):
    code: str
    message: str
```

### `src/clarateach/shared/utils.py`

```python
import secrets
import string
from datetime import datetime, timedelta

# Characters for generating codes (excluding confusing ones like 0/O, 1/I/l)
CODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
ODEHASH_ALPHABET = "abcdefghjkmnpqrstuvwxyz23456789"


def generate_workshop_id() -> str:
    """Generate a workshop ID like 'ws-abc123'."""
    suffix = "".join(secrets.choice(string.ascii_lowercase + string.digits) for _ in range(6))
    return f"ws-{suffix}"


def generate_workshop_code() -> str:
    """Generate a workshop code like 'CLAUDE-XY9Z'."""
    suffix = "".join(secrets.choice(CODE_ALPHABET) for _ in range(4))
    return f"CLAUDE-{suffix}"


def generate_odehash() -> str:
    """Generate a reconnect code like 'x7k2m'."""
    return "".join(secrets.choice(ODEHASH_ALPHABET) for _ in range(5))


def utc_now() -> datetime:
    """Get current UTC time."""
    return datetime.utcnow()


def token_expiry(hours: int = 24) -> int:
    """Get token expiry timestamp."""
    return int((datetime.utcnow() + timedelta(hours=hours)).timestamp())
```

---

## Portal API

### `src/clarateach/portal/config.py`

```python
from functools import lru_cache
from pathlib import Path
from typing import Optional

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # Server
    host: str = "0.0.0.0"
    port: int = 3000
    debug: bool = False

    # GCP
    gcp_project: str
    gcp_zone: str = "us-central1-a"

    # JWT
    jwt_private_key: Optional[str] = None
    jwt_private_key_file: Optional[Path] = None
    jwt_public_key: Optional[str] = None
    jwt_public_key_file: Optional[Path] = None
    jwt_algorithm: str = "RS256"
    jwt_expiry_hours: int = 24

    # CORS
    cors_origins: list[str] = ["http://localhost:5173"]

    # Workshop
    workshop_domain: str = "clarateach.io"
    vm_machine_type: str = "e2-standard-8"
    vm_image: str = "ubuntu-os-cloud/ubuntu-2204-lts"
    vm_disk_size: int = 100
    use_spot_vms: bool = False

    class Config:
        env_prefix = "CLARATEACH_"
        env_file = ".env"

    @property
    def private_key(self) -> str:
        if self.jwt_private_key:
            return self.jwt_private_key
        if self.jwt_private_key_file and self.jwt_private_key_file.exists():
            return self.jwt_private_key_file.read_text()
        raise ValueError("JWT private key not configured")

    @property
    def public_key(self) -> str:
        if self.jwt_public_key:
            return self.jwt_public_key
        if self.jwt_public_key_file and self.jwt_public_key_file.exists():
            return self.jwt_public_key_file.read_text()
        raise ValueError("JWT public key not configured")


@lru_cache
def get_settings() -> Settings:
    return Settings()
```

### `src/clarateach/portal/main.py`

```python
import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from clarateach.portal.config import get_settings
from clarateach.portal.routes import health, session, workshop

app = FastAPI(
    title="ClaraTeach Portal API",
    version="0.1.0",
)


def create_app() -> FastAPI:
    settings = get_settings()

    # CORS
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Routes
    app.include_router(health.router, prefix="/api", tags=["health"])
    app.include_router(workshop.router, prefix="/api", tags=["workshops"])
    app.include_router(session.router, prefix="/api", tags=["sessions"])

    return app


def run() -> None:
    settings = get_settings()
    uvicorn.run(
        "clarateach.portal.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
    )


if __name__ == "__main__":
    run()
```

### `src/clarateach/portal/routes/health.py`

```python
from fastapi import APIRouter

router = APIRouter()


@router.get("/health")
async def health_check() -> dict[str, str]:
    return {"status": "ok"}
```

### `src/clarateach/portal/routes/workshop.py`

```python
from fastapi import APIRouter, Depends, HTTPException, status

from clarateach.portal.services.compute import ComputeService, get_compute_service
from clarateach.portal.services.secrets import SecretsService, get_secrets_service
from clarateach.shared.models import (
    CreateWorkshopRequest,
    CreateWorkshopResponse,
    ListWorkshopsResponse,
    StartWorkshopResponse,
    StopWorkshopResponse,
    Workshop,
    WorkshopStatus,
)
from clarateach.shared.utils import generate_workshop_code, generate_workshop_id, utc_now

router = APIRouter()


@router.get("/workshops", response_model=ListWorkshopsResponse)
async def list_workshops(
    compute: ComputeService = Depends(get_compute_service),
) -> ListWorkshopsResponse:
    """List all workshops."""
    workshops = await compute.list_workshops()
    return ListWorkshopsResponse(workshops=workshops)


@router.get("/workshops/{workshop_id}", response_model=dict)
async def get_workshop(
    workshop_id: str,
    compute: ComputeService = Depends(get_compute_service),
) -> dict:
    """Get a single workshop by ID."""
    workshop = await compute.get_workshop(workshop_id)
    if not workshop:
        raise HTTPException(status_code=404, detail="Workshop not found")
    return {"workshop": workshop}


@router.post(
    "/workshops",
    response_model=CreateWorkshopResponse,
    status_code=status.HTTP_201_CREATED,
)
async def create_workshop(
    request: CreateWorkshopRequest,
    compute: ComputeService = Depends(get_compute_service),
    secrets: SecretsService = Depends(get_secrets_service),
) -> CreateWorkshopResponse:
    """Create a new workshop."""
    # Generate identifiers
    workshop_id = generate_workshop_id()
    code = generate_workshop_code()

    # Store API key in Secret Manager
    secret_ref = await secrets.store_api_key(workshop_id, request.api_key)

    # Create workshop record
    workshop = Workshop(
        id=workshop_id,
        name=request.name,
        code=code,
        seats=request.seats,
        status=WorkshopStatus.CREATED,
        created_at=utc_now(),
    )

    # Store in compute service (or cache for MVP)
    await compute.create_workshop(workshop, secret_ref)

    return CreateWorkshopResponse(workshop=workshop)


@router.post(
    "/workshops/{workshop_id}/start",
    response_model=StartWorkshopResponse,
    status_code=status.HTTP_202_ACCEPTED,
)
async def start_workshop(
    workshop_id: str,
    compute: ComputeService = Depends(get_compute_service),
) -> StartWorkshopResponse:
    """Start a workshop (provision VM)."""
    workshop = await compute.get_workshop(workshop_id)
    if not workshop:
        raise HTTPException(status_code=404, detail="Workshop not found")

    if workshop.status == WorkshopStatus.RUNNING:
        raise HTTPException(status_code=400, detail="Workshop is already running")

    # Provision VM
    updated_workshop = await compute.provision_workspace(workshop_id)
    return StartWorkshopResponse(workshop=updated_workshop)


@router.post("/workshops/{workshop_id}/stop", response_model=StopWorkshopResponse)
async def stop_workshop(
    workshop_id: str,
    compute: ComputeService = Depends(get_compute_service),
    secrets: SecretsService = Depends(get_secrets_service),
) -> StopWorkshopResponse:
    """Stop a workshop (destroy VM)."""
    workshop = await compute.get_workshop(workshop_id)
    if not workshop:
        raise HTTPException(status_code=404, detail="Workshop not found")

    if workshop.status != WorkshopStatus.RUNNING:
        raise HTTPException(status_code=400, detail="Workshop is not running")

    # Destroy VM
    await compute.destroy_workspace(workshop_id)

    # Delete API key
    await secrets.delete_api_key(workshop_id)

    return StopWorkshopResponse(success=True)
```

### `src/clarateach/portal/routes/session.py`

```python
from fastapi import APIRouter, Depends, HTTPException

from clarateach.portal.services.compute import ComputeService, get_compute_service
from clarateach.portal.services.jwt import JWTService, get_jwt_service
from clarateach.shared.models import JoinRequest, JoinResponse, TokenPayload, WorkshopStatus
from clarateach.shared.utils import generate_odehash, token_expiry

router = APIRouter()


@router.post("/join", response_model=JoinResponse)
async def join_workshop(
    request: JoinRequest,
    compute: ComputeService = Depends(get_compute_service),
    jwt_service: JWTService = Depends(get_jwt_service),
) -> JoinResponse:
    """Join a workshop as a learner."""
    # Find workshop by code
    workshop = await compute.find_workshop_by_code(request.code.upper())
    if not workshop:
        raise HTTPException(status_code=404, detail="Workshop not found")

    if workshop.status != WorkshopStatus.RUNNING:
        raise HTTPException(status_code=400, detail="Workshop has not started yet")

    # Assign or retrieve seat
    if request.odehash:
        # Reconnecting learner
        seat = await compute.get_seat_by_odehash(workshop.id, request.odehash)
        if seat is None:
            raise HTTPException(
                status_code=404,
                detail="Session not found. Try joining as new learner.",
            )
        odehash = request.odehash
    else:
        # New learner
        odehash = generate_odehash()
        seat = await compute.assign_seat(workshop.id, odehash)
        if seat is None:
            raise HTTPException(status_code=400, detail="Workshop is full")

    # Create JWT token
    payload = TokenPayload(
        workshop_id=workshop.id,
        container_id=f"c-{seat:02d}",
        seat=seat,
        odehash=odehash,
        vm_ip=workshop.vm_ip or "",
        exp=token_expiry(),
    )

    token = jwt_service.create_token(payload)

    return JoinResponse(
        token=token,
        endpoint=workshop.endpoint or "",
        odehash=odehash,
        seat=seat,
    )
```

### `src/clarateach/portal/services/compute.py`

```python
import asyncio
import json
from functools import lru_cache
from typing import Optional

from google.cloud import compute_v1

from clarateach.portal.config import Settings, get_settings
from clarateach.shared.models import Workshop, WorkshopStatus


class ComputeService:
    def __init__(self, settings: Settings) -> None:
        self.settings = settings
        self.instances_client = compute_v1.InstancesClient()
        self.operations_client = compute_v1.ZoneOperationsClient()
        self.project = settings.gcp_project
        self.zone = settings.gcp_zone

    async def list_workshops(self) -> list[Workshop]:
        """List all workshops from GCP instances."""
        request = compute_v1.ListInstancesRequest(
            project=self.project,
            zone=self.zone,
            filter='labels.type="clarateach-workshop"',
        )

        workshops = []
        for instance in self.instances_client.list(request=request):
            workshops.append(self._instance_to_workshop(instance))
        return workshops

    async def get_workshop(self, workshop_id: str) -> Optional[Workshop]:
        """Get a workshop by ID."""
        try:
            request = compute_v1.GetInstanceRequest(
                project=self.project,
                zone=self.zone,
                instance=f"clarateach-{workshop_id}",
            )
            instance = self.instances_client.get(request=request)
            return self._instance_to_workshop(instance)
        except Exception:
            return None

    async def find_workshop_by_code(self, code: str) -> Optional[Workshop]:
        """Find a workshop by its code."""
        request = compute_v1.ListInstancesRequest(
            project=self.project,
            zone=self.zone,
            filter=f'labels.type="clarateach-workshop" AND labels.code="{code.lower()}"',
        )

        for instance in self.instances_client.list(request=request):
            return self._instance_to_workshop(instance)
        return None

    async def create_workshop(self, workshop: Workshop, secret_ref: str) -> None:
        """Create a workshop record (for MVP, this is a placeholder)."""
        # In MVP, we don't create the VM until start
        # Could use a cache or Firestore here
        pass

    async def provision_workspace(self, workshop_id: str) -> Workshop:
        """Provision a VM for the workshop."""
        vm_name = f"clarateach-{workshop_id}"
        startup_script = self._generate_startup_script(workshop_id)

        instance = compute_v1.Instance(
            name=vm_name,
            machine_type=f"zones/{self.zone}/machineTypes/{self.settings.vm_machine_type}",
            labels={
                "type": "clarateach-workshop",
                "workshop-id": workshop_id,
            },
            disks=[
                compute_v1.AttachedDisk(
                    boot=True,
                    auto_delete=True,
                    initialize_params=compute_v1.AttachedDiskInitializeParams(
                        source_image=f"projects/{self.settings.vm_image}",
                        disk_size_gb=self.settings.vm_disk_size,
                        disk_type=f"zones/{self.zone}/diskTypes/pd-ssd",
                    ),
                )
            ],
            network_interfaces=[
                compute_v1.NetworkInterface(
                    network="global/networks/default",
                    access_configs=[
                        compute_v1.AccessConfig(
                            name="External NAT",
                            type_="ONE_TO_ONE_NAT",
                        )
                    ],
                )
            ],
            metadata=compute_v1.Metadata(
                items=[
                    compute_v1.Items(key="startup-script", value=startup_script),
                    compute_v1.Items(key="workshop-id", value=workshop_id),
                    compute_v1.Items(key="seats-map", value="{}"),
                ]
            ),
            service_accounts=[
                compute_v1.ServiceAccount(
                    scopes=["https://www.googleapis.com/auth/cloud-platform"]
                )
            ],
        )

        if self.settings.use_spot_vms:
            instance.scheduling = compute_v1.Scheduling(
                preemptible=True,
                automatic_restart=False,
            )

        request = compute_v1.InsertInstanceRequest(
            project=self.project,
            zone=self.zone,
            instance_resource=instance,
        )

        operation = self.instances_client.insert(request=request)
        await self._wait_for_operation(operation.name)

        # Get the created instance
        return await self.get_workshop(workshop_id)  # type: ignore

    async def destroy_workspace(self, workshop_id: str) -> None:
        """Destroy the VM for a workshop."""
        request = compute_v1.DeleteInstanceRequest(
            project=self.project,
            zone=self.zone,
            instance=f"clarateach-{workshop_id}",
        )

        operation = self.instances_client.delete(request=request)
        await self._wait_for_operation(operation.name)

    async def assign_seat(self, workshop_id: str, odehash: str) -> Optional[int]:
        """Assign a seat to a learner."""
        vm_name = f"clarateach-{workshop_id}"

        # Get current metadata
        request = compute_v1.GetInstanceRequest(
            project=self.project,
            zone=self.zone,
            instance=vm_name,
        )
        instance = self.instances_client.get(request=request)

        # Parse seats map
        seats_map: dict[str, int] = {}
        max_seats = 10

        for item in instance.metadata.items:
            if item.key == "seats-map":
                seats_map = json.loads(item.value)
            elif item.key == "max-seats":
                max_seats = int(item.value)

        # Find next available seat
        used_seats = set(seats_map.values())
        seat = 1
        while seat in used_seats and seat <= max_seats:
            seat += 1

        if seat > max_seats:
            return None

        # Assign seat
        seats_map[odehash] = seat

        # Update metadata
        new_items = [
            item for item in instance.metadata.items if item.key != "seats-map"
        ]
        new_items.append(
            compute_v1.Items(key="seats-map", value=json.dumps(seats_map))
        )

        set_request = compute_v1.SetMetadataInstanceRequest(
            project=self.project,
            zone=self.zone,
            instance=vm_name,
            metadata_resource=compute_v1.Metadata(
                fingerprint=instance.metadata.fingerprint,
                items=new_items,
            ),
        )

        self.instances_client.set_metadata(request=set_request)
        return seat

    async def get_seat_by_odehash(
        self, workshop_id: str, odehash: str
    ) -> Optional[int]:
        """Get seat number for a given odehash."""
        vm_name = f"clarateach-{workshop_id}"

        request = compute_v1.GetInstanceRequest(
            project=self.project,
            zone=self.zone,
            instance=vm_name,
        )
        instance = self.instances_client.get(request=request)

        for item in instance.metadata.items:
            if item.key == "seats-map":
                seats_map = json.loads(item.value)
                return seats_map.get(odehash)

        return None

    async def _wait_for_operation(self, operation_name: str) -> None:
        """Wait for a GCP operation to complete."""
        while True:
            request = compute_v1.GetZoneOperationRequest(
                project=self.project,
                zone=self.zone,
                operation=operation_name,
            )
            operation = self.operations_client.get(request=request)

            if operation.status == compute_v1.Operation.Status.DONE:
                if operation.error:
                    raise Exception(f"Operation failed: {operation.error}")
                return

            await asyncio.sleep(2)

    def _instance_to_workshop(self, instance: compute_v1.Instance) -> Workshop:
        """Convert a GCP instance to a Workshop model."""
        vm_ip = None
        if instance.network_interfaces:
            access_configs = instance.network_interfaces[0].access_configs
            if access_configs:
                vm_ip = access_configs[0].nat_i_p

        status = (
            WorkshopStatus.RUNNING
            if instance.status == "RUNNING"
            else WorkshopStatus.STOPPED
        )

        # Extract from labels/metadata
        workshop_id = instance.labels.get("workshop-id", "")
        code = instance.labels.get("code", "").upper()
        name = "Workshop"
        seats = 10

        for item in instance.metadata.items:
            if item.key == "workshop-name":
                name = item.value
            elif item.key == "max-seats":
                seats = int(item.value)

        return Workshop(
            id=workshop_id or instance.name.replace("clarateach-", ""),
            name=name,
            code=code,
            seats=seats,
            status=status,
            created_at=instance.creation_timestamp,
            vm_name=instance.name,
            vm_ip=vm_ip,
            endpoint=f"https://{vm_ip}" if vm_ip else None,
        )

    def _generate_startup_script(self, workshop_id: str) -> str:
        """Generate the VM startup script."""
        return f"""#!/bin/bash
set -e

# Install Docker
curl -fsSL https://get.docker.com | sh

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Create workspace directory
mkdir -p /opt/clarateach
cd /opt/clarateach

# Download and start services
# (In production, pull from Artifact Registry)

echo "ClaraTeach workspace ready for {workshop_id}"
"""


@lru_cache
def get_compute_service() -> ComputeService:
    return ComputeService(get_settings())
```

### `src/clarateach/portal/services/secrets.py`

```python
from functools import lru_cache

from google.cloud import secretmanager

from clarateach.portal.config import Settings, get_settings


class SecretsService:
    def __init__(self, settings: Settings) -> None:
        self.settings = settings
        self.client = secretmanager.SecretManagerServiceClient()
        self.project = settings.gcp_project

    async def store_api_key(self, workshop_id: str, api_key: str) -> str:
        """Store an API key in Secret Manager."""
        parent = f"projects/{self.project}"
        secret_id = f"clarateach-{workshop_id}-apikey"

        # Create secret
        secret = self.client.create_secret(
            request={
                "parent": parent,
                "secret_id": secret_id,
                "secret": {"replication": {"automatic": {}}},
            }
        )

        # Add secret version
        self.client.add_secret_version(
            request={
                "parent": secret.name,
                "payload": {"data": api_key.encode("utf-8")},
            }
        )

        return secret.name

    async def get_api_key(self, workshop_id: str) -> str:
        """Retrieve an API key from Secret Manager."""
        name = f"projects/{self.project}/secrets/clarateach-{workshop_id}-apikey/versions/latest"
        response = self.client.access_secret_version(request={"name": name})
        return response.payload.data.decode("utf-8")

    async def delete_api_key(self, workshop_id: str) -> None:
        """Delete an API key from Secret Manager."""
        name = f"projects/{self.project}/secrets/clarateach-{workshop_id}-apikey"
        try:
            self.client.delete_secret(request={"name": name})
        except Exception:
            pass  # Ignore if already deleted


@lru_cache
def get_secrets_service() -> SecretsService:
    return SecretsService(get_settings())
```

### `src/clarateach/portal/services/jwt.py`

```python
from functools import lru_cache

from jose import jwt

from clarateach.portal.config import Settings, get_settings
from clarateach.shared.models import TokenPayload


class JWTService:
    def __init__(self, settings: Settings) -> None:
        self.settings = settings
        self.private_key = settings.private_key
        self.public_key = settings.public_key
        self.algorithm = settings.jwt_algorithm

    def create_token(self, payload: TokenPayload) -> str:
        """Create a JWT token."""
        return jwt.encode(
            payload.model_dump(),
            self.private_key,
            algorithm=self.algorithm,
        )

    def verify_token(self, token: str) -> TokenPayload:
        """Verify and decode a JWT token."""
        data = jwt.decode(
            token,
            self.public_key,
            algorithms=[self.algorithm],
        )
        return TokenPayload(**data)


@lru_cache
def get_jwt_service() -> JWTService:
    return JWTService(get_settings())
```

---

## Workspace Server

### `src/clarateach/workspace/main.py`

```python
import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from clarateach.workspace.routes import files, terminal

app = FastAPI(title="ClaraTeach Workspace Server", version="0.1.0")


def create_app() -> FastAPI:
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    app.include_router(terminal.router, prefix="/terminal", tags=["terminal"])
    app.include_router(files.router, prefix="/files", tags=["files"])

    return app


def run() -> None:
    import os

    port = int(os.environ.get("PORT", "3001"))
    uvicorn.run(
        "clarateach.workspace.main:app",
        host="0.0.0.0",
        port=port,
        reload=os.environ.get("DEBUG", "false").lower() == "true",
    )


if __name__ == "__main__":
    run()
```

### `src/clarateach/workspace/routes/terminal.py`

```python
import asyncio
import json
import os
import pty
import select
import struct
import fcntl
import termios

from fastapi import APIRouter, WebSocket, WebSocketDisconnect

router = APIRouter()


@router.websocket("")
async def terminal_websocket(websocket: WebSocket) -> None:
    """WebSocket endpoint for terminal access."""
    await websocket.accept()

    # Fork a PTY connected to tmux
    master_fd, slave_fd = pty.openpty()

    pid = os.fork()
    if pid == 0:
        # Child process
        os.setsid()
        os.dup2(slave_fd, 0)
        os.dup2(slave_fd, 1)
        os.dup2(slave_fd, 2)
        os.close(master_fd)
        os.close(slave_fd)

        workspace_dir = os.environ.get("WORKSPACE_DIR", "/workspace")
        os.chdir(workspace_dir)

        os.environ["TERM"] = "xterm-256color"

        # Attach to tmux session
        os.execvp("tmux", ["tmux", "attach-session", "-t", "main"])

    os.close(slave_fd)

    # Set non-blocking
    flags = fcntl.fcntl(master_fd, fcntl.F_GETFL)
    fcntl.fcntl(master_fd, fcntl.F_SETFL, flags | os.O_NONBLOCK)

    async def read_pty() -> None:
        """Read from PTY and send to WebSocket."""
        while True:
            try:
                await asyncio.sleep(0.01)
                r, _, _ = select.select([master_fd], [], [], 0)
                if r:
                    data = os.read(master_fd, 1024)
                    if data:
                        await websocket.send_json({
                            "type": "output",
                            "data": data.decode("utf-8", errors="replace"),
                        })
            except Exception:
                break

    async def write_pty() -> None:
        """Read from WebSocket and write to PTY."""
        try:
            while True:
                raw = await websocket.receive_text()
                message = json.loads(raw)

                if message["type"] == "input":
                    os.write(master_fd, message["data"].encode("utf-8"))
                elif message["type"] == "resize":
                    cols = message["cols"]
                    rows = message["rows"]
                    winsize = struct.pack("HHHH", rows, cols, 0, 0)
                    fcntl.ioctl(master_fd, termios.TIOCSWINSZ, winsize)

        except WebSocketDisconnect:
            pass

    try:
        await asyncio.gather(read_pty(), write_pty())
    finally:
        os.close(master_fd)
        os.waitpid(pid, os.WNOHANG)
```

### `src/clarateach/workspace/routes/files.py`

```python
import os
from datetime import datetime
from pathlib import Path
from typing import Optional

from fastapi import APIRouter, HTTPException, Query

from clarateach.shared.models import (
    FileInfo,
    ListFilesResponse,
    ReadFileResponse,
    WriteFileRequest,
)

router = APIRouter()

WORKSPACE_DIR = Path(os.environ.get("WORKSPACE_DIR", "/workspace"))


def validate_path(requested_path: str) -> Path:
    """Validate that the path is within the workspace directory."""
    # Normalize and resolve the path
    if requested_path.startswith("/workspace"):
        requested_path = requested_path[len("/workspace") :]
    if requested_path.startswith("/"):
        requested_path = requested_path[1:]

    full_path = (WORKSPACE_DIR / requested_path).resolve()

    # Security check
    if not str(full_path).startswith(str(WORKSPACE_DIR)):
        raise HTTPException(status_code=403, detail="Access denied")

    return full_path


@router.get("", response_model=ListFilesResponse)
async def list_files(
    path: Optional[str] = Query(default="/workspace"),
) -> ListFilesResponse:
    """List files in a directory."""
    full_path = validate_path(path or "/workspace")

    if not full_path.exists():
        raise HTTPException(status_code=404, detail="Directory not found")

    if not full_path.is_dir():
        raise HTTPException(status_code=400, detail="Path is not a directory")

    files = []
    for entry in full_path.iterdir():
        stat = entry.stat()
        files.append(
            FileInfo(
                name=entry.name,
                path=str(Path("/workspace") / entry.relative_to(WORKSPACE_DIR)),
                is_directory=entry.is_dir(),
                size=stat.st_size,
                modified_at=datetime.fromtimestamp(stat.st_mtime),
            )
        )

    # Sort: directories first, then by name
    files.sort(key=lambda f: (not f.is_directory, f.name.lower()))

    return ListFilesResponse(files=files)


@router.get("/{file_path:path}", response_model=ReadFileResponse)
async def read_file(file_path: str) -> ReadFileResponse:
    """Read a file's contents."""
    full_path = validate_path(file_path)

    if not full_path.exists():
        raise HTTPException(status_code=404, detail="File not found")

    if full_path.is_dir():
        raise HTTPException(status_code=400, detail="Path is a directory")

    try:
        content = full_path.read_text(encoding="utf-8")
        return ReadFileResponse(content=content, encoding="utf-8")
    except UnicodeDecodeError:
        # Binary file - return as base64
        import base64

        content = base64.b64encode(full_path.read_bytes()).decode("ascii")
        return ReadFileResponse(content=content, encoding="base64")


@router.put("/{file_path:path}")
async def write_file(file_path: str, request: WriteFileRequest) -> dict[str, bool]:
    """Write content to a file."""
    full_path = validate_path(file_path)

    # Create parent directories if needed
    full_path.parent.mkdir(parents=True, exist_ok=True)

    if request.encoding == "base64":
        import base64

        content = base64.b64decode(request.content)
        full_path.write_bytes(content)
    else:
        full_path.write_text(request.content, encoding="utf-8")

    return {"success": True}


@router.delete("/{file_path:path}")
async def delete_file(file_path: str) -> dict[str, bool]:
    """Delete a file."""
    full_path = validate_path(file_path)

    if not full_path.exists():
        raise HTTPException(status_code=404, detail="File not found")

    if full_path.is_dir():
        import shutil

        shutil.rmtree(full_path)
    else:
        full_path.unlink()

    return {"success": True}
```

---

## CLI Tool

### `src/clarateach/cli/main.py`

```python
import typer
from rich.console import Console

from clarateach.cli.commands import workshop

app = typer.Typer(
    name="clarateach",
    help="ClaraTeach CLI for instructors",
    no_args_is_help=True,
)

console = Console()

# Add subcommands
app.add_typer(workshop.app, name="workshop")


@app.callback()
def main() -> None:
    """ClaraTeach CLI for managing workshops."""
    pass


if __name__ == "__main__":
    app()
```

### `src/clarateach/cli/commands/workshop.py`

```python
import os
from typing import Optional

import httpx
import typer
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn
from rich.table import Table

app = typer.Typer(help="Workshop management commands")
console = Console()

API_URL = os.environ.get("CLARATEACH_API_URL", "https://api.clarateach.io")


def get_client() -> httpx.Client:
    return httpx.Client(base_url=API_URL, timeout=60.0)


@app.command("list")
def list_workshops() -> None:
    """List all workshops."""
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        progress.add_task("Loading workshops...", total=None)

        with get_client() as client:
            response = client.get("/api/workshops")
            response.raise_for_status()
            data = response.json()

    workshops = data["workshops"]

    if not workshops:
        console.print("[dim]No workshops found.[/dim]")
        return

    table = Table(title="Workshops")
    table.add_column("ID", style="cyan")
    table.add_column("Name")
    table.add_column("Code", style="green")
    table.add_column("Seats")
    table.add_column("Status")
    table.add_column("Endpoint")

    for w in workshops:
        status_style = "green" if w["status"] == "running" else "dim"
        table.add_row(
            w["id"],
            w["name"],
            w["code"],
            str(w["seats"]),
            f"[{status_style}]{w['status']}[/{status_style}]",
            w.get("endpoint", "-"),
        )

    console.print(table)


@app.command("create")
def create_workshop(
    name: Optional[str] = typer.Option(None, "--name", "-n", help="Workshop name"),
    seats: int = typer.Option(10, "--seats", "-s", help="Number of seats"),
    api_key: Optional[str] = typer.Option(
        None, "--api-key", "-k", help="Claude API key"
    ),
) -> None:
    """Create a new workshop."""
    # Interactive prompts for missing values
    if not name:
        name = typer.prompt("Workshop name")
    if not api_key:
        api_key = typer.prompt("Claude API key", hide_input=True)

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        progress.add_task("Creating workshop...", total=None)

        with get_client() as client:
            response = client.post(
                "/api/workshops",
                json={"name": name, "seats": seats, "api_key": api_key},
            )
            response.raise_for_status()
            data = response.json()

    workshop = data["workshop"]

    console.print("\n[green]✓ Workshop created![/green]\n")
    console.print(f"  [bold]Workshop ID:[/bold] {workshop['id']}")
    console.print(f"  [bold]Code:[/bold] [green]{workshop['code']}[/green]")
    console.print(f"  [bold]Seats:[/bold] {workshop['seats']}")
    console.print(
        f"\n[dim]Run `clarateach workshop start {workshop['id']}` to provision.[/dim]"
    )


@app.command("start")
def start_workshop(workshop_id: str = typer.Argument(..., help="Workshop ID")) -> None:
    """Start a workshop (provision VM)."""
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        progress.add_task("Provisioning workspace...", total=None)

        with get_client() as client:
            response = client.post(f"/api/workshops/{workshop_id}/start")
            response.raise_for_status()
            data = response.json()

    workshop = data["workshop"]

    console.print("\n[green]✓ Workshop running![/green]\n")
    console.print(f"  [bold]Endpoint:[/bold] [cyan]{workshop['endpoint']}[/cyan]")
    console.print(f"  [bold]Code:[/bold] [green]{workshop['code']}[/green]")
    console.print("\nShare the code with learners to join.")


@app.command("stop")
def stop_workshop(workshop_id: str = typer.Argument(..., help="Workshop ID")) -> None:
    """Stop a workshop (destroy VM)."""
    confirm = typer.confirm(
        "This will destroy all learner environments. Continue?", default=False
    )
    if not confirm:
        console.print("Cancelled.")
        raise typer.Abort()

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        progress.add_task("Stopping workshop...", total=None)

        with get_client() as client:
            response = client.post(f"/api/workshops/{workshop_id}/stop")
            response.raise_for_status()

    console.print("[green]✓ Workshop stopped[/green]")


@app.command("info")
def workshop_info(workshop_id: str = typer.Argument(..., help="Workshop ID")) -> None:
    """Get detailed information about a workshop."""
    with get_client() as client:
        response = client.get(f"/api/workshops/{workshop_id}")
        response.raise_for_status()
        data = response.json()

    workshop = data["workshop"]

    console.print(f"\n[bold]Workshop: {workshop['name']}[/bold]\n")
    console.print(f"  ID:       {workshop['id']}")
    console.print(f"  Code:     [green]{workshop['code']}[/green]")
    console.print(f"  Seats:    {workshop['seats']}")
    console.print(f"  Status:   {workshop['status']}")
    if workshop.get("endpoint"):
        console.print(f"  Endpoint: [cyan]{workshop['endpoint']}[/cyan]")
    if workshop.get("vm_ip"):
        console.print(f"  VM IP:    {workshop['vm_ip']}")
    console.print(f"  Created:  {workshop['created_at']}")
```

---

## Docker Configuration

### `containers/workspace/Dockerfile`

```dockerfile
FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    tmux \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*

# Install Claude CLI
RUN npm install -g @anthropic-ai/claude-code

# Install Python dependencies
WORKDIR /opt/workspace-server
COPY pyproject.toml ./
RUN pip install --no-cache-dir .

# Copy application code
COPY src/clarateach/workspace ./clarateach/workspace
COPY src/clarateach/shared ./clarateach/shared

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
ENV PYTHONPATH=/opt/workspace-server

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
exec python -m clarateach.workspace.main
```

---

## Testing

### `tests/conftest.py`

```python
import pytest
from fastapi.testclient import TestClient

from clarateach.portal.main import app


@pytest.fixture
def portal_client() -> TestClient:
    return TestClient(app)
```

### `tests/portal/test_workshop.py`

```python
from fastapi.testclient import TestClient


def test_list_workshops_empty(portal_client: TestClient) -> None:
    response = portal_client.get("/api/workshops")
    assert response.status_code == 200
    assert response.json() == {"workshops": []}


def test_create_workshop(portal_client: TestClient) -> None:
    response = portal_client.post(
        "/api/workshops",
        json={
            "name": "Test Workshop",
            "seats": 5,
            "api_key": "sk-ant-test-key-12345",
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert "workshop" in data
    assert data["workshop"]["name"] == "Test Workshop"
    assert data["workshop"]["seats"] == 5
    assert data["workshop"]["code"].startswith("CLAUDE-")
```

### `tests/shared/test_utils.py`

```python
from clarateach.shared.utils import (
    generate_odehash,
    generate_workshop_code,
    generate_workshop_id,
)


def test_generate_workshop_id() -> None:
    id1 = generate_workshop_id()
    id2 = generate_workshop_id()

    assert id1.startswith("ws-")
    assert len(id1) == 9  # ws- + 6 chars
    assert id1 != id2


def test_generate_workshop_code() -> None:
    code = generate_workshop_code()

    assert code.startswith("CLAUDE-")
    assert len(code) == 11  # CLAUDE- + 4 chars


def test_generate_odehash() -> None:
    odehash = generate_odehash()

    assert len(odehash) == 5
    # Should not contain confusing characters
    for char in odehash:
        assert char not in "oil0"
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
      dockerfile: containers/portal/Dockerfile
    ports:
      - "3000:3000"
    environment:
      - CLARATEACH_DEBUG=true
      - CLARATEACH_GCP_PROJECT=${GCP_PROJECT}
      - CLARATEACH_JWT_PRIVATE_KEY_FILE=/secrets/jwt.key
      - CLARATEACH_JWT_PUBLIC_KEY_FILE=/secrets/jwt.key.pub
    volumes:
      - ./src:/app/src
      - ./secrets:/secrets:ro

  web:
    build:
      context: ./web
    ports:
      - "5173:5173"
    volumes:
      - ./web/src:/app/src
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
      - DEBUG=true
    volumes:
      - workspace-data:/workspace

volumes:
  workspace-data:
```

---

## Summary

This Python implementation provides:

1. **FastAPI-based Portal API** with async support and Pydantic models
2. **Workspace server** with PTY handling and file API
3. **CLI tool** using Typer with rich output
4. **Shared models** using Pydantic for type safety
5. **GCP integration** using google-cloud libraries
6. **Docker containers** for deployment
7. **Comprehensive tests** with pytest

The Python implementation follows the same architecture as the TypeScript version but uses Python-idiomatic patterns and libraries.
