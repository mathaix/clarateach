# ClaraTeach - Infrastructure & Deployment

This document covers the infrastructure setup, Terraform configuration, and deployment procedures for ClaraTeach.

---

## Infrastructure Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              GCP Project                                    │
│                           (clarateach-prod)                                 │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         ADMIN STACK                                 │    │
│  │                                                                     │    │
│  │   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐            │    │
│  │   │  Cloud Run  │    │   Secret    │    │  Artifact   │            │    │
│  │   │  (Portal)   │    │   Manager   │    │  Registry   │            │    │
│  │   └─────────────┘    └─────────────┘    └─────────────┘            │    │
│  │                                                                     │    │
│  │   ┌─────────────┐    ┌─────────────┐                               │    │
│  │   │  Cloud DNS  │    │   Cloud     │                               │    │
│  │   │             │    │   Logging   │                               │    │
│  │   └─────────────┘    └─────────────┘                               │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                       WORKSPACE STACK                               │    │
│  │                      (Per Workshop)                                 │    │
│  │                                                                     │    │
│  │   ┌─────────────────────────────────────────────────────────────┐   │    │
│  │   │                  Compute Engine VM                          │   │    │
│  │   │                  (e2-standard-8)                            │   │    │
│  │   │                                                             │   │    │
│  │   │   ┌─────────┐  ┌─────────┐  ┌─────────┐     ┌─────────┐    │   │    │
│  │   │   │  Caddy  │  │  C-01   │  │  C-02   │ ... │  C-10   │    │   │    │
│  │   │   │ (Proxy) │  │(Learner)│  │(Learner)│     │(Learner)│    │   │    │
│  │   │   └─────────┘  └─────────┘  └─────────┘     └─────────┘    │   │    │
│  │   └─────────────────────────────────────────────────────────────┘   │    │
│  │                                                                     │    │
│  │   Created/destroyed dynamically per workshop                        │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Terraform Structure

```
infrastructure/
├── main.tf                     # Root module
├── variables.tf                # Input variables
├── outputs.tf                  # Output values
├── versions.tf                 # Provider versions
├── terraform.tfvars.example    # Example variables
│
├── modules/
│   ├── admin/                  # Admin stack resources
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   ├── outputs.tf
│   │   ├── cloud_run.tf
│   │   ├── secrets.tf
│   │   ├── dns.tf
│   │   └── registry.tf
│   │
│   └── workspace/              # Workspace stack resources
│       ├── main.tf
│       ├── variables.tf
│       ├── outputs.tf
│       ├── vm.tf
│       ├── firewall.tf
│       └── templates/
│           └── startup.sh.tpl
│
├── environments/
│   ├── dev/
│   │   ├── main.tf
│   │   ├── terraform.tfvars
│   │   └── backend.tf
│   │
│   └── prod/
│       ├── main.tf
│       ├── terraform.tfvars
│       └── backend.tf
│
└── scripts/
    ├── setup-gcp.sh            # Initial GCP setup
    └── generate-keys.sh        # Generate JWT keys
```

---

## Terraform Configuration

### `infrastructure/versions.tf`

```hcl
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}
```

### `infrastructure/variables.tf`

```hcl
variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "GCP zone"
  type        = string
  default     = "us-central1-a"
}

variable "domain" {
  description = "Domain for ClaraTeach"
  type        = string
  default     = "clarateach.io"
}

variable "environment" {
  description = "Environment (dev, prod)"
  type        = string
  default     = "dev"
}
```

### `infrastructure/main.tf`

```hcl
# Enable required APIs
resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "compute.googleapis.com",
    "secretmanager.googleapis.com",
    "artifactregistry.googleapis.com",
    "dns.googleapis.com",
    "cloudbuild.googleapis.com",
  ])

  service            = each.key
  disable_on_destroy = false
}

# Admin stack
module "admin" {
  source = "./modules/admin"

  project_id  = var.project_id
  region      = var.region
  domain      = var.domain
  environment = var.environment

  depends_on = [google_project_service.apis]
}

# Output for CLI usage
output "portal_url" {
  value = module.admin.portal_url
}

output "api_url" {
  value = module.admin.api_url
}
```

---

## Admin Stack Module

### `infrastructure/modules/admin/main.tf`

```hcl
variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "domain" {
  type = string
}

variable "environment" {
  type = string
}

locals {
  service_name = "clarateach-portal-${var.environment}"
}
```

### `infrastructure/modules/admin/registry.tf`

```hcl
# Artifact Registry for container images
resource "google_artifact_registry_repository" "images" {
  location      = var.region
  repository_id = "clarateach-${var.environment}"
  format        = "DOCKER"
  description   = "ClaraTeach container images"
}

output "registry_url" {
  value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}"
}
```

### `infrastructure/modules/admin/secrets.tf`

```hcl
# JWT signing key
resource "google_secret_manager_secret" "jwt_private_key" {
  secret_id = "clarateach-jwt-private-key-${var.environment}"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "jwt_public_key" {
  secret_id = "clarateach-jwt-public-key-${var.environment}"

  replication {
    auto {}
  }
}

# Note: Secret versions are created manually or via CI/CD
# Do not store actual keys in Terraform state

output "jwt_private_key_secret" {
  value = google_secret_manager_secret.jwt_private_key.id
}

output "jwt_public_key_secret" {
  value = google_secret_manager_secret.jwt_public_key.id
}
```

### `infrastructure/modules/admin/cloud_run.tf`

```hcl
# Service account for Cloud Run
resource "google_service_account" "portal" {
  account_id   = "clarateach-portal-${var.environment}"
  display_name = "ClaraTeach Portal Service Account"
}

# Grant permissions
resource "google_project_iam_member" "portal_compute" {
  project = var.project_id
  role    = "roles/compute.admin"
  member  = "serviceAccount:${google_service_account.portal.email}"
}

resource "google_project_iam_member" "portal_secrets" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.portal.email}"
}

# Cloud Run service
resource "google_cloud_run_v2_service" "portal" {
  name     = local.service_name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.portal.email

    scaling {
      min_instance_count = 0
      max_instance_count = 10
    }

    containers {
      image = "${var.region}-docker.pkg.dev/${var.project_id}/clarateach-${var.environment}/portal:latest"

      ports {
        container_port = 3000
      }

      env {
        name  = "NODE_ENV"
        value = "production"
      }

      env {
        name  = "GCP_PROJECT"
        value = var.project_id
      }

      env {
        name  = "GCP_ZONE"
        value = "${var.region}-a"
      }

      env {
        name = "JWT_PRIVATE_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.jwt_private_key.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "JWT_PUBLIC_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.jwt_public_key.secret_id
            version = "latest"
          }
        }
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }
}

# Allow unauthenticated access
resource "google_cloud_run_service_iam_member" "portal_invoker" {
  location = google_cloud_run_v2_service.portal.location
  service  = google_cloud_run_v2_service.portal.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "portal_url" {
  value = google_cloud_run_v2_service.portal.uri
}

output "api_url" {
  value = google_cloud_run_v2_service.portal.uri
}
```

### `infrastructure/modules/admin/dns.tf`

```hcl
# Managed DNS zone
resource "google_dns_managed_zone" "main" {
  name        = "clarateach-${var.environment}"
  dns_name    = "${var.domain}."
  description = "ClaraTeach DNS zone"
}

# Portal subdomain
resource "google_dns_record_set" "portal" {
  name         = "portal.${google_dns_managed_zone.main.dns_name}"
  type         = "CNAME"
  ttl          = 300
  managed_zone = google_dns_managed_zone.main.name
  rrdatas      = ["ghs.googlehosted.com."]
}

# API subdomain
resource "google_dns_record_set" "api" {
  name         = "api.${google_dns_managed_zone.main.dns_name}"
  type         = "CNAME"
  ttl          = 300
  managed_zone = google_dns_managed_zone.main.name
  rrdatas      = ["ghs.googlehosted.com."]
}

output "dns_name_servers" {
  value = google_dns_managed_zone.main.name_servers
}
```

---

## Workspace Stack Module

### `infrastructure/modules/workspace/variables.tf`

```hcl
variable "project_id" {
  type = string
}

variable "zone" {
  type = string
}

variable "workshop_id" {
  description = "Workshop ID"
  type        = string
}

variable "seats" {
  description = "Number of learner seats"
  type        = number
  default     = 10
}

variable "machine_type" {
  description = "VM machine type"
  type        = string
  default     = "e2-standard-8"
}

variable "disk_size" {
  description = "Boot disk size in GB"
  type        = number
  default     = 100
}

variable "use_spot" {
  description = "Use spot/preemptible VMs"
  type        = bool
  default     = true
}

variable "api_key_secret" {
  description = "Secret Manager reference for API key"
  type        = string
}

variable "workspace_image" {
  description = "Container image for workspace"
  type        = string
}

variable "neko_image" {
  description = "Container image for neko browser"
  type        = string
}
```

### `infrastructure/modules/workspace/vm.tf`

```hcl
# Service account for workspace VM
resource "google_service_account" "workspace" {
  account_id   = "clarateach-ws-${var.workshop_id}"
  display_name = "ClaraTeach Workspace ${var.workshop_id}"
}

# Grant permissions
resource "google_project_iam_member" "workspace_secrets" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.workspace.email}"
}

resource "google_project_iam_member" "workspace_logs" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.workspace.email}"
}

# Compute Engine VM
resource "google_compute_instance" "workspace" {
  name         = "clarateach-${var.workshop_id}"
  machine_type = var.machine_type
  zone         = var.zone

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
      size  = var.disk_size
      type  = "pd-ssd"
    }
  }

  network_interface {
    network = "default"

    access_config {
      // Ephemeral public IP
    }
  }

  metadata = {
    workshop-id    = var.workshop_id
    seats          = var.seats
    seats-map      = "{}"
    api-key-secret = var.api_key_secret
  }

  metadata_startup_script = templatefile("${path.module}/templates/startup.sh.tpl", {
    workshop_id     = var.workshop_id
    seats           = var.seats
    workspace_image = var.workspace_image
    neko_image      = var.neko_image
    api_key_secret  = var.api_key_secret
    project_id      = var.project_id
  })

  labels = {
    type        = "clarateach-workshop"
    workshop-id = var.workshop_id
  }

  service_account {
    email  = google_service_account.workspace.email
    scopes = ["cloud-platform"]
  }

  scheduling {
    preemptible       = var.use_spot
    automatic_restart = !var.use_spot
  }

  tags = ["clarateach-workspace"]
}

output "vm_ip" {
  value = google_compute_instance.workspace.network_interface[0].access_config[0].nat_ip
}

output "vm_name" {
  value = google_compute_instance.workspace.name
}
```

### `infrastructure/modules/workspace/firewall.tf`

```hcl
# Firewall rules for workspace VMs
resource "google_compute_firewall" "workspace_http" {
  name    = "clarateach-workspace-http"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["clarateach-workspace"]
}

resource "google_compute_firewall" "workspace_ssh" {
  name    = "clarateach-workspace-ssh"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  # Restrict to known IPs in production
  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["clarateach-workspace"]
}
```

### `infrastructure/modules/workspace/templates/startup.sh.tpl`

```bash
#!/bin/bash
set -e

# Log to Cloud Logging
exec > >(tee /var/log/startup.log) 2>&1

echo "Starting ClaraTeach workspace setup..."

# Install Docker
curl -fsSL https://get.docker.com | sh

# Add ubuntu user to docker group
usermod -aG docker ubuntu

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" \
  -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Install gcloud CLI (for Secret Manager)
apt-get update && apt-get install -y apt-transport-https ca-certificates gnupg
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" \
  | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg \
  | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -
apt-get update && apt-get install -y google-cloud-cli

# Get API key from Secret Manager
API_KEY=$(gcloud secrets versions access latest --secret="${api_key_secret}" --project="${project_id}")

# Create workspace directory
mkdir -p /opt/clarateach
cd /opt/clarateach

# Generate docker-compose.yml
cat > docker-compose.yml << 'COMPOSE_EOF'
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy-data:/data
      - caddy-config:/config
    networks:
      - frontend

%{ for i in range(1, seats + 1) ~}
  workspace-${format("%02d", i)}:
    image: ${workspace_image}
    restart: unless-stopped
    environment:
      - CLAUDE_API_KEY=$${API_KEY}
      - WORKSPACE_DIR=/workspace
      - SEAT=${format("%02d", i)}
    volumes:
      - workspace-${format("%02d", i)}-data:/workspace
    networks:
      - frontend
    deploy:
      resources:
        limits:
          cpus: '1.5'
          memory: 3G

  neko-${format("%02d", i)}:
    image: ${neko_image}
    restart: unless-stopped
    environment:
      - NEKO_SCREEN=1280x720@30
      - NEKO_PASSWORD=clarateach
      - NEKO_CONTROL_PROTECTION=true
    shm_size: 2gb
    networks:
      - frontend
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 2G

%{ endfor ~}

networks:
  frontend:

volumes:
  caddy-data:
  caddy-config:
%{ for i in range(1, seats + 1) ~}
  workspace-${format("%02d", i)}-data:
%{ endfor ~}
COMPOSE_EOF

# Generate Caddyfile
cat > Caddyfile << 'CADDY_EOF'
{
    email admin@clarateach.io
    auto_https off
}

:80, :443 {
%{ for i in range(1, seats + 1) ~}
    handle /vm/${format("%02d", i)}/terminal* {
        reverse_proxy workspace-${format("%02d", i)}:3001
    }

    handle /vm/${format("%02d", i)}/files* {
        reverse_proxy workspace-${format("%02d", i)}:3002
    }

    handle /vm/${format("%02d", i)}/browser* {
        reverse_proxy neko-${format("%02d", i)}:8080
    }

%{ endfor ~}
    respond "ClaraTeach Workspace"
}
CADDY_EOF

# Export API key for docker-compose
export API_KEY

# Pull images
docker-compose pull

# Start services
docker-compose up -d

echo "ClaraTeach workspace ready!"
echo "Workshop ID: ${workshop_id}"
echo "Seats: ${seats}"
```

---

## Environment Configuration

### `infrastructure/environments/prod/main.tf`

```hcl
terraform {
  backend "gcs" {
    bucket = "clarateach-terraform-state"
    prefix = "prod"
  }
}

module "clarateach" {
  source = "../../"

  project_id  = "clarateach-prod"
  region      = "us-central1"
  zone        = "us-central1-a"
  domain      = "clarateach.io"
  environment = "prod"
}
```

### `infrastructure/environments/prod/terraform.tfvars`

```hcl
project_id  = "clarateach-prod"
region      = "us-central1"
zone        = "us-central1-a"
domain      = "clarateach.io"
environment = "prod"
```

---

## Deployment Procedures

### Initial Setup

```bash
#!/bin/bash
# scripts/setup-gcp.sh

PROJECT_ID="clarateach-prod"
REGION="us-central1"

# Create project (if needed)
gcloud projects create $PROJECT_ID --name="ClaraTeach Production"

# Set project
gcloud config set project $PROJECT_ID

# Enable billing (manual step required)
echo "Please enable billing for project $PROJECT_ID in GCP Console"

# Create Terraform state bucket
gsutil mb -l $REGION gs://${PROJECT_ID}-terraform-state

# Enable versioning
gsutil versioning set on gs://${PROJECT_ID}-terraform-state

# Create service account for Terraform
gcloud iam service-accounts create terraform \
  --display-name="Terraform Service Account"

# Grant permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:terraform@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/owner"

# Create and download key
gcloud iam service-accounts keys create terraform-key.json \
  --iam-account=terraform@${PROJECT_ID}.iam.gserviceaccount.com

echo "Setup complete. Set GOOGLE_APPLICATION_CREDENTIALS to terraform-key.json"
```

### Generate JWT Keys

```bash
#!/bin/bash
# scripts/generate-keys.sh

# Generate RSA key pair
openssl genrsa -out jwt-private.pem 2048
openssl rsa -in jwt-private.pem -pubout -out jwt-public.pem

# Store in Secret Manager
gcloud secrets create clarateach-jwt-private-key-prod \
  --data-file=jwt-private.pem

gcloud secrets create clarateach-jwt-public-key-prod \
  --data-file=jwt-public.pem

# Clean up local files
rm jwt-private.pem jwt-public.pem

echo "JWT keys stored in Secret Manager"
```

### Deploy Admin Stack

```bash
# From infrastructure/environments/prod

# Initialize Terraform
terraform init

# Plan changes
terraform plan -out=tfplan

# Apply changes
terraform apply tfplan
```

### Build and Push Container Images

```bash
#!/bin/bash
# Deploy portal image

PROJECT_ID="clarateach-prod"
REGION="us-central1"
REGISTRY="${REGION}-docker.pkg.dev/${PROJECT_ID}/clarateach-prod"

# Build portal
docker build -t ${REGISTRY}/portal:latest -f apps/portal/Dockerfile .
docker push ${REGISTRY}/portal:latest

# Build workspace
docker build -t ${REGISTRY}/workspace:latest -f containers/workspace/Dockerfile .
docker push ${REGISTRY}/workspace:latest

# Deploy to Cloud Run
gcloud run deploy clarateach-portal-prod \
  --image=${REGISTRY}/portal:latest \
  --region=$REGION \
  --platform=managed
```

### CI/CD with GitHub Actions

```yaml
# .github/workflows/deploy.yml

name: Deploy

on:
  push:
    branches: [main]

env:
  PROJECT_ID: clarateach-prod
  REGION: us-central1
  REGISTRY: us-central1-docker.pkg.dev/clarateach-prod/clarateach-prod

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest

    permissions:
      contents: read
      id-token: write

    steps:
      - uses: actions/checkout@v4

      - name: Authenticate to GCP
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.WIF_PROVIDER }}
          service_account: ${{ secrets.WIF_SERVICE_ACCOUNT }}

      - name: Set up Cloud SDK
        uses: google-github-actions/setup-gcloud@v2

      - name: Configure Docker
        run: gcloud auth configure-docker ${{ env.REGION }}-docker.pkg.dev

      - name: Build Portal
        run: |
          docker build -t ${{ env.REGISTRY }}/portal:${{ github.sha }} \
            -f apps/portal/Dockerfile .
          docker push ${{ env.REGISTRY }}/portal:${{ github.sha }}

      - name: Build Workspace
        run: |
          docker build -t ${{ env.REGISTRY }}/workspace:${{ github.sha }} \
            -f containers/workspace/Dockerfile .
          docker push ${{ env.REGISTRY }}/workspace:${{ github.sha }}

      - name: Deploy Portal
        run: |
          gcloud run deploy clarateach-portal-prod \
            --image=${{ env.REGISTRY }}/portal:${{ github.sha }} \
            --region=${{ env.REGION }} \
            --platform=managed

      - name: Tag as latest
        run: |
          docker tag ${{ env.REGISTRY }}/portal:${{ github.sha }} \
            ${{ env.REGISTRY }}/portal:latest
          docker push ${{ env.REGISTRY }}/portal:latest

          docker tag ${{ env.REGISTRY }}/workspace:${{ github.sha }} \
            ${{ env.REGISTRY }}/workspace:latest
          docker push ${{ env.REGISTRY }}/workspace:latest
```

---

## Cost Optimization

### Spot/Preemptible VMs

```hcl
# Use spot instances for workspace VMs
scheduling {
  preemptible       = true
  automatic_restart = false
}
```

**Savings:** ~60-70% compared to regular VMs

**Trade-off:** VMs can be terminated with 30 seconds notice

### Cloud Run Scaling

```hcl
scaling {
  min_instance_count = 0  # Scale to zero when idle
  max_instance_count = 10
}
```

**Cost:** $0 when no traffic

### Auto-shutdown Idle Workshops

```bash
# Cron job to shut down idle workshops
# Run every hour

#!/bin/bash
# Check for workshops with no WebSocket connections for > 2 hours
# Trigger shutdown via API

gcloud compute instances list \
  --filter="labels.type=clarateach-workshop AND status=RUNNING" \
  --format="value(name,creationTimestamp)" | while read name created; do

  # Check if workshop has been idle
  # (Implementation depends on metrics/logging)

  echo "Checking $name..."
done
```

---

## Monitoring

### Cloud Monitoring Dashboard

```yaml
# monitoring/dashboard.yaml
displayName: ClaraTeach Dashboard

gridLayout:
  widgets:
    - title: Cloud Run Requests
      xyChart:
        dataSets:
          - timeSeriesQuery:
              timeSeriesFilter:
                filter: resource.type="cloud_run_revision"
                aggregation:
                  alignmentPeriod: 60s
                  perSeriesAligner: ALIGN_RATE

    - title: Active Workspace VMs
      scorecard:
        timeSeriesQuery:
          timeSeriesFilter:
            filter: resource.type="gce_instance" AND metric.type="compute.googleapis.com/instance/uptime"

    - title: Container CPU Usage
      xyChart:
        dataSets:
          - timeSeriesQuery:
              timeSeriesFilter:
                filter: resource.type="gce_instance"
                aggregation:
                  alignmentPeriod: 60s
```

### Alerts

```hcl
# Alert for high error rate
resource "google_monitoring_alert_policy" "error_rate" {
  display_name = "ClaraTeach High Error Rate"

  conditions {
    display_name = "Error rate > 5%"

    condition_threshold {
      filter          = "resource.type=\"cloud_run_revision\" AND metric.type=\"run.googleapis.com/request_count\""
      comparison      = "COMPARISON_GT"
      threshold_value = 0.05
      duration        = "300s"

      aggregations {
        alignment_period   = "60s"
        per_series_aligner = "ALIGN_RATE"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]
}
```

---

## Backup & Recovery

### Terraform State

```hcl
# State stored in GCS with versioning
terraform {
  backend "gcs" {
    bucket = "clarateach-terraform-state"
    prefix = "prod"
  }
}
```

### Secret Manager

Secrets are automatically versioned. To restore:

```bash
gcloud secrets versions access <version-number> \
  --secret=clarateach-jwt-private-key-prod
```

### Workshop Data

Workshop data is ephemeral by design. For persistent storage (future):

```hcl
# Attach persistent disk to VM
resource "google_compute_disk" "workshop_data" {
  name = "clarateach-${var.workshop_id}-data"
  size = 50
  type = "pd-ssd"
  zone = var.zone
}
```

---

## Security Checklist

- [ ] Enable VPC Service Controls
- [ ] Configure IAM with least privilege
- [ ] Enable Cloud Armor for DDoS protection
- [ ] Configure Secret Manager access policies
- [ ] Enable Cloud Audit Logs
- [ ] Set up security scanning for containers
- [ ] Configure firewall rules appropriately
- [ ] Enable HTTPS everywhere
- [ ] Rotate JWT keys periodically
- [ ] Review and remove unused service accounts
