#!/bin/bash

# Preempt Kubernetes Deployment Script
# This script deploys all components in the correct order
# Similar to "docker-compose up"

set -e  # Exit on error

echo "🚀 Starting Preempt Kubernetes Deployment..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl not found. Please install kubectl first."
    exit 1
fi

# Check if Kubernetes is running
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Kubernetes cluster not accessible. Make sure Docker Desktop Kubernetes is enabled."
    exit 1
fi

# Step 1: Create namespace and config
echo ""
echo "📦 Step 1/7: Creating namespace and configuration..."
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f persistent-volumes.yaml

# Step 2: Deploy databases
echo ""
echo "🗄️  Step 2/7: Deploying MySQL and Redis..."
kubectl apply -f mysql-deployment.yaml
kubectl apply -f redis-deployment.yaml

echo "⏳ Waiting for MySQL to be ready..."
kubectl wait --for=condition=ready pod -l app=mysql -n preempt --timeout=120s

echo "⏳ Waiting for Redis to be ready..."
kubectl wait --for=condition=ready pod -l app=redis -n preempt --timeout=60s

# Step 3: Run migrations
echo ""
echo "🔄 Step 3/7: Running database migrations..."
kubectl apply -f migrate-job.yaml
echo "⏳ Waiting for migrations to complete..."
kubectl wait --for=condition=complete job/migrate -n preempt --timeout=180s

# Step 4: Seed database
echo ""
echo "🌱 Step 4/7: Seeding database with locations..."
kubectl apply -f seed-job.yaml
echo "⏳ Waiting for seed job to complete..."
kubectl wait --for=condition=complete job/seed -n preempt --timeout=180s

# Step 5: Deploy backend services
echo ""
echo "⚙️  Step 5/7: Deploying backend services..."
kubectl apply -f api-deployment.yaml
kubectl apply -f store-deployment.yaml
kubectl apply -f ml-trainer-deployment.yaml

# Step 6: Deploy scheduled jobs
echo ""
echo "⏰ Step 6/7: Deploying CronJobs..."
kubectl apply -f collector-cronjob.yaml
kubectl apply -f detector-cronjob.yaml

# Step 7: Deploy frontend
echo ""
echo "🌐 Step 7/7: Deploying frontend..."
kubectl apply -f frontend-deployment.yaml

# Wait for deployments to be ready
echo ""
echo "⏳ Waiting for deployments to be ready..."
kubectl wait --for=condition=available deployment/api -n preempt --timeout=120s
kubectl wait --for=condition=available deployment/store -n preempt --timeout=120s
kubectl wait --for=condition=available deployment/ml-trainer -n preempt --timeout=120s
kubectl wait --for=condition=available deployment/frontend -n preempt --timeout=120s

# Display status
echo ""
echo "✅ Deployment complete!"
echo ""
echo "📊 Current status:"
kubectl get pods -n preempt
echo ""
echo "🌐 Access frontend at: http://localhost:30000"
echo ""
echo "📝 Useful commands:"
echo "  • View all resources:  kubectl get all -n preempt"
echo "  • View logs:          kubectl logs -f deployment/api -n preempt"
echo "  • Delete everything:  kubectl delete namespace preempt"
echo ""
