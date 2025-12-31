# ClaraTeach Deployment Plan for Google Cloud with CI/CD

## 1. High-Level Goal

To deploy the ClaraTeach application to Google Cloud Platform (GCP) using a fully automated CI/CD pipeline. The pipeline will automatically build, test, and deploy both the Admin Stack (Portal API and frontend) and the Workspace Stack base image whenever changes are pushed to the main branch of the Git repository.

## 2. Prerequisites

*   **GCP Project:** A new or existing GCP project.
*   **Billing Enabled:** Billing must be enabled for the project.
*   **Domain Name:** A registered domain name (e.g., `clarateach.io`) for the application.
*   **Google Cloud SDK (`gcloud`):** Installed and authenticated on a local machine for initial setup.
*   **Terraform:** Installed locally for initial infrastructure setup.
*   **Git Repository:** A Git repository hosted on a platform supported by Cloud Build (e.g., GitHub, Bitbucket Cloud, Cloud Source Repositories). This plan assumes GitHub.

## 3. Core GCP Services

*   **Cloud Build:** For CI/CD pipelines.
*   **Cloud Run:** To host the stateless Admin Stack (Portal API and static frontend assets).
*   **Artifact Registry:** To store Docker images (Workspace container and `neko` images).
*   **Compute Engine (GCE):** To run the ephemeral Workspace Stacks (VMs for workshops).
*   **Cloud DNS:** To manage DNS records for the application and dynamically for each workshop VM.
*   **Secret Manager:** To store sensitive data (API keys, JWT signing keys, etc.).
*   **IAM (Identity and Access Management):** To manage permissions for services and users.

## 4. CI/CD Pipeline with Cloud Build

The CI/CD pipeline will be defined in a `cloudbuild.yaml` file at the root of the repository. It will be triggered on every push to the `main` branch.

### Pipeline Stages:

1.  **Checkout:** Cloud Build automatically checks out the source code.
2.  **Lint & Test:**
    *   Run linting and unit tests for the `frontend`.
    *   Run linting and unit tests for the `workspace/server`.
3.  **Build Docker Images:**
    *   Build the `workspace/server` Docker image using the `workspace/Dockerfile`.
    *   Tag the image with the Git commit SHA.
    *   Push the image to Artifact Registry.
    *   (Optional) If a custom `neko` image is needed, build and push it as well.
4.  **Build Frontend:**
    *   Build the React frontend for production (`npm run build`).
5.  **Deploy Admin Stack:**
    *   Deploy the Portal API to Cloud Run. (Note: This is in a separate repository and will have its own pipeline. This plan focuses on the `ClaraTeach` repository's components).
    *   Deploy the `frontend/dist` directory to a separate Cloud Run service configured to serve static assets, or to a Cloud Storage bucket with a CDN in front of it.
6.  **Update GCE VM Template (if applicable):**
    *   If using GCE instance templates for workshops, create a new version of the template that uses the newly built Docker image from Artifact Registry.

### `cloudbuild.yaml` Example:

```yaml
steps:
  # -------------------- Frontend Build & Test --------------------
  - name: 'gcr.io/cloud-builders/npm'
    id: 'Frontend Test'
    dir: 'frontend'
    args: ['install']
  - name: 'gcr.io/cloud-builders/npm'
    dir: 'frontend'
    args: ['run', 'lint']
  - name: 'gcr.io/cloud-builders/npm'
    dir: 'frontend'
    args: ['run', 'test']
  - name: 'gcr.io/cloud-builders/npm'
    id: 'Frontend Build'
    dir: 'frontend'
    args: ['run', 'build']

  # -------------------- Workspace Server Build & Test --------------------
  - name: 'gcr.io/cloud-builders/npm'
    id: 'Server Test'
    dir: 'workspace/server'
    args: ['install']
  - name: 'gcr.io/cloud-builders/npm'
    dir: 'workspace/server'
    args: ['run', 'lint']
  - name: 'gcr.io/cloud-builders/npm'
    dir: 'workspace/server'
    args: ['run', 'test']

  # -------------------- Docker Build & Push --------------------
  - name: 'gcr.io/cloud-builders/docker'
    id: 'Build Workspace Image'
    args: [
        'build',
        '-t',
        'us-central1-docker.pkg.dev/$PROJECT_ID/clarateach/workspace-server:$COMMIT_SHA',
        '.',
        '-f',
        'workspace/Dockerfile',
      ]
  - name: 'gcr.io/cloud-builders/docker'
    id: 'Push Workspace Image'
    args: ['push', 'us-central1-docker.pkg.dev/$PROJECT_ID/clarateach/workspace-server:$COMMIT_SHA']

  # -------------------- Frontend Deploy to Cloud Run --------------------
  - name: 'gcr.io/google.com/cloudsdktool/cloud-sdk'
    id: 'Deploy Frontend'
    entrypoint: gcloud
    args:
      - 'run'
      - 'deploy'
      - 'clarateach-frontend'
      - '--image=gcr.io/cloud-run/container-server' # A generic container to serve static files
      - '--region=us-central1'
      - '--platform=managed'
      - '--allow-unauthenticated'
      - '--set-env-vars=SOURCE_DIR=frontend/dist' # Custom env var for a custom server
    # A better approach is to use a dedicated static site serving container
    # or upload to a GCS bucket with Cloud CDN.

images:
  - 'us-central1-docker.pkg.dev/$PROJECT_ID/clarateach/workspace-server:$COMMIT_SHA'
```

## 5. Infrastructure as Code (Terraform)

Terraform should be used to provision and manage all the static cloud infrastructure. This includes:

*   **IAM Roles and Service Accounts:**
    *   Service account for Cloud Build with permissions to push to Artifact Registry and deploy to Cloud Run.
    *   Service account for the Portal API with permissions to manage GCE instances and Cloud DNS records.
    *   Service account for the Workspace VMs with permissions to pull from Artifact Registry and access Secret Manager.
*   **Artifact Registry Repository:** For Docker images.
*   **Cloud Run Services:** For the frontend and (in the other repo) the Portal API.
*   **Cloud DNS Zone:** The managed zone for the domain.
*   **Secret Manager Secrets:** Placeholders for secrets.
*   **Firewall Rules:** For the Workspace VMs.

The Terraform code should be stored in a separate repository or a dedicated directory within the main repository.

## 6. Deployment Strategy

*   **Admin Stack:** The Portal API and frontend are deployed to Cloud Run, which provides a serverless, scalable, and managed environment. Deployments are handled automatically by the Cloud Build pipeline.
*   **Workspace Stack:**
    1.  The `portal-api` (not in this repo) receives a request to start a workshop.
    2.  It uses the GCP SDK to programmatically create a new GCE VM.
    3.  The VM is created from a startup script or instance template that:
        *   Installs Docker.
        *   Authenticates to Artifact Registry.
        *   Pulls the `workspace-server` and `neko` Docker images.
        *   Pulls the Caddy configuration from GCS or has it baked into the image.
        *   Starts the containers using `docker-compose` with the `docker-compose.yml` file (not the local one).
    4.  The Portal API also creates a DNS `A` record in Cloud DNS to point a subdomain (e.g., `ws-abc.clarateach.io`) to the new VM's IP address.
    5.  Caddy on the VM automatically obtains an SSL certificate for the subdomain.

## 7. Security Considerations

*   **Principle of Least Privilege:** All service accounts should have the minimum necessary IAM permissions.
*   **Secret Management:** All secrets (API keys, JWT signing keys, passwords) must be stored in Secret Manager. Do not store them in environment variables in Terraform files or check them into Git.
*   **Container Security:**
    *   Use minimal base images for Docker containers.
    *   Scan images for vulnerabilities using Artifact Registry's built-in scanning.
    *   Apply the security options from `ARCHITECTURE.md` (`no-new-privileges`, etc.) in the `docker-compose.yml`.
*   **Network Security:**
    *   Use the firewall rules defined in `ARCHITECTURE.md` to restrict traffic to the Workspace VMs.
    *   Isolate learner containers from each other using Docker networks.
*   **Authentication:** The JWT-based authentication model is good. Ensure the `JWKS_URL` is configured correctly for the `workspace-server`.

## 8. Monitoring and Logging

*   **Cloud Logging:** All services (Cloud Run, GCE, Cloud Build) should be configured to send logs to Cloud Logging.
*   **Cloud Monitoring:**
    *   Set up dashboards to monitor key metrics for the Admin Stack and Workspace VMs (CPU, memory, network).
    *   Create alerting policies for error rates, high latency, or high resource utilization, as defined in `ARCHITECTURE.md`.
*   **Health Checks:** Both the Portal API and the `workspace-server` should have `/health` endpoints that are monitored by uptime checks.
