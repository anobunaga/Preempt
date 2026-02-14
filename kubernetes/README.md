# Kubernetes Deployment Guide for Preempt Weather Monitoring System

This directory contains Kubernetes manifests to run the Preempt weather monitoring system on Kubernetes, specifically configured for Docker Desktop on macOS.

## Table of Contents
- [Overview](#overview)
- [Architecture Changes from Docker Compose](#architecture-changes-from-docker-compose)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Detailed Component Breakdown](#detailed-component-breakdown)
- [Key Kubernetes Concepts Used](#key-kubernetes-concepts-used)
- [Accessing Services](#accessing-services)
- [Scaling](#scaling)
- [Troubleshooting](#troubleshooting)
- [Clean Up](#clean-up)

## Overview

This Kubernetes deployment replicates the functionality of the Docker Compose setup but leverages Kubernetes primitives for better scalability, reliability, and production-readiness.

### System Architecture
```
┌─────────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                          │
│                      (Docker Desktop)                           │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │  Frontend    │  │   API (x2)   │  │    Store     │         │
│  │  (NodePort)  │──│  Deployment  │──│  Deployment  │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
│         │                  │                  │                 │
│         │                  ▼                  ▼                 │
│         │          ┌──────────────┐  ┌──────────────┐         │
│         │          │    MySQL     │  │    Redis     │         │
│         │          │  (StatefulDB)│  │   (Cache)    │         │
│         │          └──────────────┘  └──────────────┘         │
│         │                  ▲                  ▲                 │
│         │                  │                  │                 │
│         │          ┌──────────────┐  ┌──────────────┐         │
│         │          │   Migrate    │  │  ML Trainer  │         │
│         │          │    (Job)     │  │  Deployment  │         │
│         │          └──────────────┘  └──────────────┘         │
│         │                                      ▲                │
│         │          ┌──────────────┐           │                │
│         └──────────│     Seed     │───────────┘                │
│                    │    (Job)     │                            │
│                    └──────────────┘                            │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  CronJobs (Every 5 minutes)                              │ │
│  │  - Collector CronJob → Fetch weather data               │ │
│  │  - Detector CronJob  → Run anomaly detection            │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Architecture Changes from Docker Compose

### 1. **Container Orchestration → Kubernetes Resources**

| Docker Compose Concept | Kubernetes Equivalent | Why the Change? |
|------------------------|----------------------|-----------------|
| `services:` | Deployments + Services | Deployments manage pod replicas, Services provide networking |
| `volumes:` | PersistentVolumeClaims | K8s separates storage from compute for flexibility |
| `networks:` | Service DNS | Automatic service discovery via DNS (e.g., `mysql:3306`) |
| `depends_on:` | Init Containers + Jobs | More explicit control over startup ordering |
| Ofelia scheduler | CronJobs | Native Kubernetes scheduling (no external scheduler needed) |
| `restart:` policies | Deployment strategy | K8s handles restarts via pod lifecycle management |

### 2. **Stateful vs Stateless Workloads**

**Docker Compose approach:**
- All services treated similarly with `restart:` policies
- Manual volume management

**Kubernetes approach:**
- **Deployments** for stateless apps (API, frontend, store, ml-trainer)
  - Can scale horizontally (replicas: 2 for API/frontend)
  - Auto-restart on failure
  - Rolling updates for zero-downtime deployments
  
- **Jobs** for one-time tasks (migrate, seed)
  - Run to completion
  - Automatically cleaned up (successfulJobsHistoryLimit: 3)
  
- **CronJobs** for scheduled tasks (collector, detector)
  - Native Kubernetes cron scheduling (replaces Ofelia)
  - `concurrencyPolicy: Forbid` prevents overlapping runs
  - Automatic cleanup of old job executions

- **PersistentVolumeClaims** for stateful data (MySQL, Redis, ML models)
  - Survive pod restarts
  - Independent lifecycle from pods

### 3. **Configuration Management**

**Docker Compose:**
```yaml
environment:
  - DB_HOST=mysql
  - DB_PASSWORD=mypassword123
volumes:
  - ./config.yaml:/app/config.yaml
```

**Kubernetes:**
```yaml
# Secrets for sensitive data (passwords, connection strings)
env:
- name: DB_PASSWORD
  valueFrom:
    secretKeyRef:
      name: mysql-secret
      key: DB_PASSWORD

# ConfigMaps for application config
volumeMounts:
- name: config
  mountPath: /app/config.yaml
  subPath: config.yaml
```

**Why?**
- **Secrets** are base64-encoded and can be encrypted at rest
- **ConfigMaps** allow updating config without rebuilding images
- Clear separation of sensitive vs non-sensitive data

### 4. **Service Discovery & Networking**

**Docker Compose:**
- Bridge network with container names as hostnames
- Port mapping to host: `ports: - "8080:8080"`

**Kubernetes:**
- **ClusterIP Services** for internal communication (MySQL, Redis, API)
  - DNS-based discovery: `mysql.preempt.svc.cluster.local` (or just `mysql`)
- **NodePort Service** for external access (frontend)
  - Accessible on Docker Desktop via `localhost:30000`

### 5. **Health Checks & Reliability**

**Docker Compose:**
```yaml
healthcheck:
  test: ["CMD", "mysqladmin", "ping"]
  interval: 10s
```

**Kubernetes:**
```yaml
livenessProbe:   # Restart pod if this fails
  exec:
    command: [mysqladmin, ping]
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:  # Remove from load balancer if not ready
  exec:
    command: [mysqladmin, ping]
  initialDelaySeconds: 10
```

**Key differences:**
- **Liveness probes** → restart unhealthy pods
- **Readiness probes** → control traffic routing (pod can be alive but not ready)
- More granular control with `initialDelaySeconds`, `failureThreshold`, etc.

### 6. **Resource Management**

**Docker Compose:** No resource limits specified

**Kubernetes:**
```yaml
resources:
  requests:      # Guaranteed resources
    memory: "256Mi"
    cpu: "200m"
  limits:        # Maximum allowed
    memory: "512Mi"
    cpu: "500m"
```

**Benefits:**
- **Requests** ensure minimum resources (scheduling guarantee)
- **Limits** prevent resource starvation
- Better multi-tenancy and cost control in production

### 7. **Scheduling Changes**

**Docker Compose (Ofelia):**
```yaml
labels:
  ofelia.enabled: "true"
  ofelia.job-exec.collect-weather.schedule: "@every 5m"
```
- Requires separate Ofelia container
- Docker-specific solution

**Kubernetes (CronJobs):**
```yaml
apiVersion: batch/v1
kind: CronJob
spec:
  schedule: "*/5 * * * *"  # Standard cron syntax
  concurrencyPolicy: Forbid
```
- Native Kubernetes primitive
- Standard cron syntax
- Better job history and failure tracking
- No external dependencies

## Prerequisites

### 1. Enable Kubernetes in Docker Desktop

1. Open Docker Desktop
2. Go to **Settings** (⚙️) → **Kubernetes**
3. Check **Enable Kubernetes**
4. Click **Apply & Restart**
5. Wait for Kubernetes to start (green indicator in bottom-left)

Verify installation:
```bash
kubectl version --client
kubectl cluster-info
```

You should see:
```
Kubernetes control plane is running at https://127.0.0.1:6443
```

### 2. Build Docker Images

Since Kubernetes pulls images from a registry (or uses local images with `imagePullPolicy: IfNotPresent`), you need to build the images first:

```bash
# Navigate to project root
cd /Users/adamnobunaga/projects/Preempt

# Build backend image (contains all Go binaries + Python ML)
docker build -t preempt-backend:latest -f Dockerfile .

# Build frontend image
docker build -t preempt-frontend:latest -f frontend/Dockerfile ./frontend
```

**Why build locally?**
- Docker Desktop's Kubernetes uses the same Docker daemon
- `imagePullPolicy: IfNotPresent` uses local images
- No need to push to a registry for local development

## Quick Start

### Option 1: Deploy Everything with One Command (Recommended)

**Using the deployment script (similar to `docker-compose up`):**

```bash
cd /Users/adamnobunaga/projects/Preempt/kubernetes

# Deploy everything in correct order
./deploy.sh

# Clean up everything (similar to `docker-compose down`)
./destroy.sh
```

The script automatically:
- Creates namespace, ConfigMaps, Secrets, and PVCs
- Deploys MySQL and Redis, waits for readiness
- Runs migrations and seed jobs
- Deploys all backend services
- Deploys CronJobs and frontend
- Shows status when complete

### Option 2: Using Kustomize (Built into kubectl)

```bash
cd /Users/adamnobunaga/projects/Preempt/kubernetes

# Deploy everything at once
kubectl apply -k .

# Note: This applies all manifests but doesn't wait for dependencies
# You may need to retry if migrations/seed fail due to timing
```

### Option 3: Manual Step-by-Step (for learning)

#### Step 1: Create Namespace and Base Resources

```bash
cd /Users/adamnobunaga/projects/Preempt/kubernetes

# Create namespace (logical isolation)
kubectl apply -f namespace.yaml

# Create ConfigMaps (application config)
kubectl apply -f configmap.yaml

# Create Secrets (passwords, connection strings)
kubectl apply -f secrets.yaml

# Create Persistent Volume Claims (storage)
kubectl apply -f persistent-volumes.yaml
```

**What happens:**
- Namespace `preempt` is created
- ConfigMap `preempt-config` contains config.yaml
- Secrets `mysql-secret` and `redis-secret` store credentials
- PVCs request storage for MySQL (10Gi), Redis (5Gi), ML models (2Gi)

### Step 2: Deploy Databases

```bash
# Deploy MySQL (stateful database)
kubectl apply -f mysql-deployment.yaml

# Deploy Redis (cache & message queue)
kubectl apply -f redis-deployment.yaml

# Wait for databases to be ready
kubectl wait --for=condition=ready pod -l app=mysql -n preempt --timeout=120s
kubectl wait --for=condition=ready pod -l app=redis -n preempt --timeout=60s
```

**What happens:**
- MySQL and Redis pods are created
- PVCs are bound to actual storage
- Health checks run until pods are ready
- Services create DNS entries: `mysql:3306`, `redis:6379`

#### Step 3: Run Database Migrations

```bash
# Run migration job (creates schema)
kubectl apply -f migrate-job.yaml

# Watch migration progress
kubectl logs -f job/migrate -n preempt

# Wait for completion
kubectl wait --for=condition=complete job/migrate -n preempt --timeout=180s
```

**What happens:**
- Job creates a pod that runs `migrate up`
- Init container waits for MySQL to be ready
- Migrations from `./migrations/` are applied
- Job completes and pod terminates (but logs remain)

#### Step 4: Seed Database with Locations

```bash
# Run seed job (imports 892 locations from CSV)
kubectl apply -f seed-job.yaml

# Watch seed progress
kubectl logs -f job/seed -n preempt

# Wait for completion
kubectl wait --for=condition=complete job/seed -n preempt --timeout=180s
```

**What happens:**
- Seed job waits for migrate job to complete
- Reads `locations_seed.csv` from hostPath mount
- Inserts 892 locations into MySQL
- Job completes

#### Step 5: Deploy Backend Services

```bash
# Deploy API server (2 replicas for load balancing)
kubectl apply -f api-deployment.yaml

# Deploy Store consumer (processes Redis stream)
kubectl apply -f store-deployment.yaml

# Deploy ML trainer (long-running ML model training)
kubectl apply -f ml-trainer-deployment.yaml
```

**What happens:**
- API deployment creates 2 pods (can handle more traffic)
- Store deployment creates 1 pod (consumer group in Redis)
- ML trainer deployment creates 1 pod
- All connect to MySQL and Redis via service DNS

#### Step 6: Deploy Scheduled Jobs

```bash
# Deploy collector CronJob (runs every 5 minutes)
kubectl apply -f collector-cronjob.yaml

# Deploy detector CronJob (runs every 5 minutes)
kubectl apply -f detector-cronjob.yaml
```

**What happens:**
- CronJobs are registered but don't run immediately
- First execution: next 5-minute mark (e.g., if deployed at 10:03, runs at 10:05)
- Each creates a Job → Pod → executes binary → terminates
- Keeps last 3 successful and 3 failed job histories

**Trigger manually (for testing):**
```bash
# Create a one-off job from CronJob
kubectl create job --from=cronjob/collector collector-manual -n preempt
kubectl logs -f job/collector-manual -n preempt
```

#### Step 7: Deploy Frontend

```bash
# Deploy frontend (2 replicas with NodePort for external access)
kubectl apply -f frontend-deployment.yaml
```

**What happens:**
- Frontend deployment creates 2 pods running Nginx
- NodePort service exposes port 30000 on host
- Access via `http://localhost:30000`

#### Step 8: Verify Everything

```bash
# Check all pods
kubectl get pods -n preempt

# Expected output:
# NAME                          READY   STATUS      RESTARTS   AGE
# api-xxxxxxxxx-xxxxx           1/1     Running     0          2m
# api-xxxxxxxxx-xxxxx           1/1     Running     0          2m
# frontend-xxxxxxxxx-xxxxx      1/1     Running     0          1m
# frontend-xxxxxxxxx-xxxxx      1/1     Running     0          1m
# ml-trainer-xxxxxxxxx-xxxxx    1/1     Running     0          2m
# mysql-xxxxxxxxx-xxxxx         1/1     Running     0          5m
# redis-xxxxxxxxx-xxxxx         1/1     Running     0          5m
# store-xxxxxxxxx-xxxxx         1/1     Running     0          2m
# migrate-xxxxx                 0/1     Completed   0          4m
# seed-xxxxx                    0/1     Completed   0          3m

# Check services
kubectl get svc -n preempt

# Check CronJobs
kubectl get cronjobs -n preempt
```

## Detailed Component Breakdown

### 1. Namespace (`namespace.yaml`)

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: preempt
```

**Purpose:** Logical isolation for all Preempt resources

**Benefits:**
- Resource organization (all Preempt pods/services in one namespace)
- RBAC (can set permissions per namespace)
- Resource quotas (limit CPU/memory per namespace)
- Easy cleanup: `kubectl delete namespace preempt` removes everything

**Docker Compose equivalent:** N/A (implicit isolation via project name)

### 2. ConfigMap (`configmap.yaml`)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: preempt-config
data:
  config.yaml: |
    weather:
      monitored_fields:
        - temperature_2m
        ...
```

**Purpose:** Store non-sensitive configuration

**Usage in pods:**
```yaml
volumeMounts:
- name: config
  mountPath: /app/config.yaml
  subPath: config.yaml  # Mount specific file, not entire configmap
```

**Benefits:**
- Update config without rebuilding images
- Share config across multiple pods
- Version control configuration changes

**Docker Compose equivalent:** `volumes: - ./config.yaml:/app/config.yaml`

### 3. Secrets (`secrets.yaml`)

```yaml
apiVersion: v1
kind: Secret
type: Opaque
stringData:  # Auto-base64 encoded
  DB_PASSWORD: mypassword123
```

**Purpose:** Store sensitive data (passwords, API keys)

**Usage in pods:**
```yaml
env:
- name: DB_PASSWORD
  valueFrom:
    secretKeyRef:
      name: mysql-secret
      key: DB_PASSWORD
```

**Security improvements over Docker Compose:**
- Base64 encoded (not plaintext in YAML)
- Can be encrypted at rest with K8s encryption providers
- RBAC controls who can read secrets
- Can integrate with external secret managers (HashiCorp Vault, AWS Secrets Manager)

**Docker Compose equivalent:** `environment: - DB_PASSWORD=mypassword123` (plaintext)

### 4. PersistentVolumeClaims (`persistent-volumes.yaml`)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mysql-pvc
spec:
  accessModes:
    - ReadWriteOnce  # Single node can mount read/write
  resources:
    requests:
      storage: 10Gi
```

**Purpose:** Request persistent storage

**How it works:**
1. PVC requests storage (10Gi)
2. Kubernetes finds/creates a PersistentVolume (PV)
3. PVC binds to PV
4. Pod mounts PVC

**Storage in Docker Desktop:**
- Uses `hostPath` provisioner (stores on Mac filesystem)
- Data survives pod restarts/deletions
- Located in Docker Desktop VM storage

**Docker Compose equivalent:** `volumes: mysql_data:/var/lib/mysql`

**Production difference:**
- In cloud (AWS/GCP/Azure), would use cloud-native storage (EBS, Persistent Disk, Azure Disk)
- Can have different storage classes (SSD, HDD, replicated)

### 5. MySQL Deployment (`mysql-deployment.yaml`)

**Key features:**

```yaml
spec:
  replicas: 1
  strategy:
    type: Recreate  # Important for stateful apps
```

**Why `Recreate` strategy?**
- MySQL uses a single RWO (ReadWriteOnce) PVC
- Can't have 2 pods writing to same PVC simultaneously
- `Recreate` ensures old pod terminates before new pod starts
- Alternative: Use StatefulSet (more advanced, for production)

**Liveness vs Readiness probes:**
```yaml
livenessProbe:   # "Is the pod healthy?"
  exec:
    command: [mysqladmin, ping, -h, localhost, -u, root, -prootpassword]
  initialDelaySeconds: 30  # Wait 30s before first check
  failureThreshold: 3      # Restart after 3 failures

readinessProbe:  # "Is the pod ready for traffic?"
  exec:
    command: [mysqladmin, ping]
  initialDelaySeconds: 10  # Ready check starts sooner
```

**What happens on failure:**
- **Liveness fails:** Pod is restarted
- **Readiness fails:** Pod is removed from Service endpoints (no traffic routed)

**Resource requests/limits:**
```yaml
resources:
  requests:
    memory: "512Mi"  # Scheduler guarantees this
    cpu: "250m"      # 0.25 CPU cores
  limits:
    memory: "1Gi"    # OOMKilled if exceeded
    cpu: "500m"      # Throttled if exceeded
```

**Service:**
```yaml
apiVersion: v1
kind: Service
spec:
  type: ClusterIP  # Internal only
  ports:
  - port: 3306
    targetPort: 3306
```

**How DNS works:**
- Service creates DNS entry: `mysql.preempt.svc.cluster.local`
- Short form works within namespace: `mysql:3306`
- Pods use this instead of IP addresses

### 6. Jobs vs CronJobs

**Job (migrate-job.yaml, seed-job.yaml):**
```yaml
apiVersion: batch/v1
kind: Job
spec:
  backoffLimit: 4  # Retry 4 times if fails
  template:
    spec:
      restartPolicy: OnFailure  # Restart on failure, not Always
```

**Lifecycle:**
1. Create Job
2. Job creates Pod
3. Pod runs to completion
4. Job marked as `Completed`
5. Pod remains (logs accessible)
6. Manual cleanup or TTL controller

**Init containers for ordering:**
```yaml
initContainers:
- name: wait-for-mysql
  image: busybox:1.35
  command:
  - sh
  - -c
  - |
    until nc -z mysql 3306; do
      echo "Waiting..."
      sleep 2
    done
```

**Why not `depends_on`?**
- Docker Compose `depends_on` is simple but limited
- Init containers allow custom readiness logic
- More explicit and debuggable

**CronJob (collector-cronjob.yaml, detector-cronjob.yaml):**
```yaml
apiVersion: batch/v1
kind: CronJob
spec:
  schedule: "*/5 * * * *"  # Standard cron: minute hour day month weekday
  concurrencyPolicy: Forbid  # Don't allow overlapping runs
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
```

**Replaces Docker Compose Ofelia:**
- No external scheduler container needed
- Native Kubernetes primitive
- Better monitoring and debugging

**Cron schedule syntax:**
```
*/5 * * * *
│  │ │ │ │
│  │ │ │ └─── Day of week (0-7, 0=Sunday)
│  │ │ └───── Month (1-12)
│  │ └─────── Day of month (1-31)
│  └───────── Hour (0-23)
└─────────── Minute (0-59)

*/5 = every 5 minutes
0 */2 * * * = every 2 hours
0 0 * * 0 = every Sunday at midnight
```

**Concurrency policies:**
- `Forbid`: Skip run if previous still running (prevents overlap)
- `Allow`: Allow multiple concurrent runs
- `Replace`: Cancel old run, start new one

### 7. Deployments (api, store, ml-trainer, frontend)

**Deployment manages ReplicaSets, ReplicaSet manages Pods:**

```
Deployment → ReplicaSet → Pod (replica 1)
                       → Pod (replica 2)
```

**Scaling:**
```bash
# Scale API to 5 replicas
kubectl scale deployment api -n preempt --replicas=5

# Auto-scale based on CPU (requires metrics-server)
kubectl autoscale deployment api -n preempt --min=2 --max=10 --cpu-percent=80
```

**Rolling updates (zero downtime):**
```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1  # At most 1 pod down during update
      maxSurge: 1        # At most 1 extra pod during update
```

**Update process:**
1. Change image: `kubectl set image deployment/api api=preempt-backend:v2 -n preempt`
2. K8s creates new pod with v2
3. Waits for readiness probe to pass
4. Terminates old pod
5. Repeat until all pods updated

**Rollback:**
```bash
kubectl rollout undo deployment/api -n preempt
kubectl rollout history deployment/api -n preempt
```

### 8. Frontend Service (NodePort)

**Why NodePort?**
```yaml
apiVersion: v1
kind: Service
spec:
  type: NodePort
  ports:
  - port: 80
    targetPort: 80
    nodePort: 30000  # Fixed port on host
```

**Service types comparison:**

| Type | Use Case | Accessibility |
|------|----------|---------------|
| **ClusterIP** (default) | Internal services (MySQL, Redis, API) | Only within cluster |
| **NodePort** | Development/testing | `localhost:30000` on Docker Desktop |
| **LoadBalancer** | Production (cloud only) | External IP via cloud LB (AWS ELB, GCP LB) |
| **ExternalName** | Alias to external service | DNS CNAME |

**Why not LoadBalancer on Docker Desktop?**
- LoadBalancer requires cloud provider integration
- Docker Desktop doesn't support external load balancers
- NodePort is perfect for local development

**Production change:**
```yaml
# In production, use LoadBalancer or Ingress
spec:
  type: LoadBalancer  # Cloud provisions external IP
  # OR use Ingress for HTTP routing
```

## Key Kubernetes Concepts Used

### 1. Labels and Selectors

**Labels** tag resources:
```yaml
metadata:
  labels:
    app: api
    tier: backend
    environment: dev
```

**Selectors** query resources:
```bash
kubectl get pods -l app=api -n preempt
kubectl get pods -l tier=backend -n preempt
kubectl delete pods -l environment=dev -n preempt  # Bulk operations
```

**Service selectors:**
```yaml
# Service routes traffic to pods with matching labels
spec:
  selector:
    app: api  # Routes to any pod with label "app: api"
```

### 2. Resource Requests vs Limits

**Requests** (guaranteed):
- Used by scheduler to place pod on node
- Node must have available resources matching requests
- Pod gets at least this much

**Limits** (maximum):
- Pod cannot exceed limits
- CPU: throttled if exceeded
- Memory: OOMKilled (Out Of Memory) if exceeded

**Best practices:**
```yaml
# CPU: compressible resource (throttling is acceptable)
requests:
  cpu: "200m"  # Guaranteed
limits:
  cpu: "500m"  # Can burst up to 500m

# Memory: non-compressible (OOM is bad)
requests:
  memory: "256Mi"  # Guaranteed
limits:
  memory: "512Mi"  # Hard limit (killed if exceeded)
```

**Setting appropriate values:**
1. Start with educated guesses
2. Monitor actual usage: `kubectl top pods -n preempt`
3. Adjust based on metrics

### 3. Health Checks

**Three types:**

1. **Liveness Probe** - "Should this pod be restarted?"
   - Checks if application is running
   - Failure → restart pod
   - Example: API endpoint returns 200 OK

2. **Readiness Probe** - "Should this pod receive traffic?"
   - Checks if application is ready to serve requests
   - Failure → remove from Service endpoints
   - Example: Database connection established

3. **Startup Probe** - "Has application started?"
   - Used for slow-starting apps
   - Delays liveness/readiness checks
   - Failure → restart pod

**Example:**
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30  # Wait 30s after container start
  periodSeconds: 10         # Check every 10s
  timeoutSeconds: 5         # Timeout after 5s
  failureThreshold: 3       # Fail after 3 consecutive failures
  successThreshold: 1       # Success after 1 success (default)

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10   # Check readiness sooner
  periodSeconds: 5
```

**Probe types:**
- `httpGet`: HTTP GET request (returns 200-399)
- `exec`: Execute command in container (exit code 0)
- `tcpSocket`: TCP connection succeeds

### 4. Init Containers

**Purpose:** Run setup tasks before main container starts

**Example (wait for MySQL):**
```yaml
initContainers:
- name: wait-for-mysql
  image: busybox:1.35
  command:
  - sh
  - -c
  - until nc -z mysql 3306; do sleep 2; done
```

**Lifecycle:**
1. Pod created
2. Init containers run sequentially
3. All init containers must succeed
4. Main containers start
5. If init container fails, pod restarts (based on restartPolicy)

**Use cases:**
- Wait for dependencies (databases, APIs)
- Clone git repos
- Generate configuration files
- Security setup (change permissions)

### 5. ConfigMaps vs Secrets

**When to use ConfigMap:**
- Application configuration (config.yaml)
- Environment-specific settings
- Non-sensitive data

**When to use Secret:**
- Passwords
- API keys
- TLS certificates
- OAuth tokens

**Mounting strategies:**

```yaml
# As environment variables
env:
- name: DB_HOST
  valueFrom:
    configMapKeyRef:
      name: preempt-config
      key: db.host

# As files
volumeMounts:
- name: config
  mountPath: /app/config.yaml
  subPath: config.yaml  # Mount single file from ConfigMap
```

## Accessing Services

### Internal Services (ClusterIP)

Services with `type: ClusterIP` are only accessible within the cluster.

**Access from your machine:**

```bash
# Port-forward to access MySQL from localhost
kubectl port-forward svc/mysql 3306:3306 -n preempt
# Now connect via: mysql -h 127.0.0.1 -u myapp -pmypassword123 preempt

# Port-forward to access API
kubectl port-forward svc/api 8080:8080 -n preempt
# Access via: http://localhost:8080

# Port-forward to access Redis
kubectl port-forward svc/redis 6379:6379 -n preempt
# Connect via: redis-cli -h 127.0.0.1
```

**Access from within cluster:**
```bash
# Exec into a pod
kubectl exec -it deployment/api -n preempt -- /bin/sh

# Inside pod, use service DNS
curl http://mysql:3306
curl http://api:8080/health
redis-cli -h redis -p 6379 ping
```

### External Service (NodePort)

Frontend uses NodePort for external access:

```yaml
spec:
  type: NodePort
  ports:
  - port: 80
    nodePort: 30000
```

**Access:**
- **Docker Desktop:** http://localhost:30000
- **Remote cluster:** http://<node-ip>:30000

**NodePort range:** 30000-32767 (default Kubernetes range)

## Scaling

### Manual Scaling

```bash
# Scale API deployment to 5 replicas
kubectl scale deployment api -n preempt --replicas=5

# Verify
kubectl get deployment api -n preempt
# NAME   READY   UP-TO-DATE   AVAILABLE   AGE
# api    5/5     5            5           10m

# Scale down
kubectl scale deployment api -n preempt --replicas=2
```

**Which services can scale horizontally?**
- ✅ **API** - stateless, can have multiple replicas
- ✅ **Frontend** - stateless, can have multiple replicas
- ⚠️ **Store** - single consumer group (scaling requires code changes)
- ❌ **MySQL** - single RWO PVC (requires StatefulSet for HA)
- ❌ **Redis** - single instance (requires Redis Sentinel/Cluster)
- ⚠️ **ML Trainer** - depends on training logic (may need locking)

### Horizontal Pod Autoscaler (HPA)

**Requires metrics-server:**
```bash
# Install metrics-server (Docker Desktop may not include it)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

**Create HPA:**
```bash
# Auto-scale API based on CPU usage
kubectl autoscale deployment api -n preempt \
  --min=2 \
  --max=10 \
  --cpu-percent=80

# Check HPA status
kubectl get hpa -n preempt
```

**HPA manifest example:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-hpa
  namespace: preempt
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 80
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## Troubleshooting

### Check Pod Status

```bash
# List all pods
kubectl get pods -n preempt

# Describe pod (see events, errors)
kubectl describe pod <pod-name> -n preempt

# Check logs
kubectl logs <pod-name> -n preempt

# Follow logs (tail -f)
kubectl logs -f <pod-name> -n preempt

# Logs from previous crashed container
kubectl logs <pod-name> -n preempt --previous

# Logs from specific container in multi-container pod
kubectl logs <pod-name> -c <container-name> -n preempt
```

### Common Issues

#### 1. **Pod stuck in `Pending`**

**Symptoms:**
```bash
kubectl get pods -n preempt
# NAME                     READY   STATUS    RESTARTS   AGE
# mysql-xxxxxxxxx-xxxxx    0/1     Pending   0          5m
```

**Causes:**
- Insufficient resources (CPU/memory)
- PVC not bound
- Node selector mismatch

**Debug:**
```bash
kubectl describe pod <pod-name> -n preempt
# Look for events like:
# - "0/1 nodes available: insufficient memory"
# - "pod has unbound immediate PersistentVolumeClaims"

# Check PVC status
kubectl get pvc -n preempt
# Should show "Bound", not "Pending"
```

**Fix:**
- Reduce resource requests
- Check storage class: `kubectl get storageclass`
- Increase Docker Desktop resources (Settings → Resources)

#### 2. **Pod stuck in `CrashLoopBackOff`**

**Symptoms:**
```bash
kubectl get pods -n preempt
# NAME                     READY   STATUS             RESTARTS   AGE
# api-xxxxxxxxx-xxxxx      0/1     CrashLoopBackOff   5          10m
```

**Causes:**
- Application crash on startup
- Missing environment variables
- Database not ready

**Debug:**
```bash
# Check logs
kubectl logs <pod-name> -n preempt

# Check previous logs (if container restarted)
kubectl logs <pod-name> -n preempt --previous

# Describe pod for restart reason
kubectl describe pod <pod-name> -n preempt
```

**Fix:**
- Fix application code
- Add init containers to wait for dependencies
- Check secrets/configmaps are mounted correctly

#### 3. **Job never completes**

**Symptoms:**
```bash
kubectl get jobs -n preempt
# NAME      COMPLETIONS   DURATION   AGE
# migrate   0/1           10m        10m
```

**Debug:**
```bash
# Check job pods
kubectl get pods -l job-name=migrate -n preempt

# Check logs
kubectl logs job/migrate -n preempt

# Describe job
kubectl describe job migrate -n preempt
```

**Common causes:**
- Application hangs
- Wrong command
- Missing files (hostPath not mounted)

#### 4. **Service not accessible**

**Debug:**
```bash
# Check service exists
kubectl get svc -n preempt

# Check endpoints (pods backing the service)
kubectl get endpoints -n preempt
# If endpoints empty, no pods match selector

# Test from within cluster
kubectl run test-pod --rm -it --image=busybox -n preempt -- sh
# Inside pod:
wget -O- http://api:8080/health

# Port-forward to test
kubectl port-forward svc/api 8080:8080 -n preempt
curl http://localhost:8080/health
```

#### 5. **Image pull errors**

**Symptoms:**
```bash
kubectl get pods -n preempt
# NAME                     READY   STATUS         RESTARTS   AGE
# api-xxxxxxxxx-xxxxx      0/1     ErrImagePull   0          1m
```

**Causes:**
- Image not built locally
- Wrong image name
- imagePullPolicy: Always (tries to pull from registry)

**Fix:**
```bash
# Build images locally
docker build -t preempt-backend:latest .
docker build -t preempt-frontend:latest ./frontend

# Verify images exist
docker images | grep preempt

# Check imagePullPolicy in manifests
# Should be: imagePullPolicy: IfNotPresent
```

### Debugging Commands

```bash
# Get all resources in namespace
kubectl get all -n preempt

# Exec into running pod
kubectl exec -it <pod-name> -n preempt -- /bin/sh

# Copy files from pod
kubectl cp <pod-name>:/app/config.yaml ./config.yaml -n preempt

# Copy files to pod
kubectl cp ./config.yaml <pod-name>:/app/config.yaml -n preempt

# Check resource usage
kubectl top pods -n preempt
kubectl top nodes

# Stream events
kubectl get events -n preempt --watch

# Get YAML of running resource
kubectl get deployment api -n preempt -o yaml

# Edit resource in-place
kubectl edit deployment api -n preempt
```

## Clean Up

### Delete specific resources

```bash
# Delete deployment
kubectl delete deployment api -n preempt

# Delete service
kubectl delete svc api -n preempt

# Delete job
kubectl delete job migrate -n preempt

# Delete cronjob
kubectl delete cronjob collector -n preempt
```

### Delete everything

```bash
# Delete entire namespace (removes all resources)
kubectl delete namespace preempt

# Confirm deletion
kubectl get namespaces
```

**What gets deleted:**
- All pods, deployments, services, jobs, cronjobs
- ConfigMaps, Secrets
- PersistentVolumeClaims (storage is deleted!)

**To preserve data:**
```bash
# Delete resources but keep PVCs
kubectl delete deployment,service,job,cronjob --all -n preempt
# PVCs remain, can be reattached later
```

## Production Considerations

This setup is optimized for **local development on Docker Desktop**. For production:

### 1. **Use StatefulSets for Stateful Apps**

Replace MySQL and Redis Deployments with StatefulSets:
- Stable network identities (mysql-0, mysql-1)
- Ordered deployment/scaling
- Stable persistent storage

### 2. **Use Ingress for HTTP Routing**

Replace NodePort with Ingress:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: preempt-ingress
spec:
  rules:
  - host: preempt.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: frontend
            port:
              number: 80
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: api
            port:
              number: 8080
```

### 3. **External Secret Management**

Use external secret managers instead of K8s Secrets:
- **AWS:** External Secrets Operator + AWS Secrets Manager
- **GCP:** Workload Identity + Secret Manager
- **Azure:** Key Vault CSI Driver
- **HashiCorp Vault:** Vault Agent Injector

### 4. **Database High Availability**

- **MySQL:** Use MySQL Operator (Vitess, Percona) or cloud RDS
- **Redis:** Use Redis Cluster or cloud ElastiCache/MemoryStore

### 5. **Observability**

Add monitoring stack (from previous conversation):
- Prometheus (metrics)
- Grafana (visualization)
- Loki (logs)
- Jaeger (distributed tracing)

### 6. **GitOps**

Use ArgoCD or Flux for declarative deployments:
- Git as source of truth
- Automatic sync
- Rollback capabilities

### 7. **Security Hardening**

- Network Policies (restrict pod-to-pod communication)
- Pod Security Standards (restrict privileged containers)
- RBAC (least-privilege access)
- Image scanning (Trivy, Snyk)

## Summary

### What You Learned

1. **Kubernetes Primitives:**
   - Namespaces, ConfigMaps, Secrets, PVCs
   - Deployments, Services, Jobs, CronJobs
   - Labels, selectors, health probes

2. **Migration from Docker Compose:**
   - Services → Deployments + Services
   - Volumes → PVCs
   - depends_on → Init Containers + Jobs
   - Ofelia → CronJobs
   - Environment variables → Secrets + ConfigMaps

3. **Key Differences:**
   - Declarative vs imperative
   - Self-healing (restarts, rescheduling)
   - Horizontal scaling
   - Rolling updates
   - Resource management

4. **Operational Skills:**
   - kubectl commands
   - Debugging pods, logs, events
   - Port-forwarding, exec
   - Scaling deployments

### Next Steps

1. **Experiment:**
   - Scale deployments up/down
   - Trigger manual CronJob runs
   - Break things and fix them
   - Update images and watch rolling updates

2. **Enhance:**
   - Add Prometheus monitoring
   - Implement Ingress
   - Add HPA for auto-scaling
   - Use Helm charts for templating

3. **Production Path:**
   - Learn StatefulSets
   - Explore cloud Kubernetes (EKS, GKE, AKS)
   - Study GitOps (ArgoCD)
   - Practice disaster recovery

4. **Certification:**
   - CKA (Certified Kubernetes Administrator)
   - CKAD (Certified Kubernetes Application Developer)

---

**Questions?** Open an issue or check [official docs](https://kubernetes.io/docs/).

**Happy Kubernetes learning! 🚀**
