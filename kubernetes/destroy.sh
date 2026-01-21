#!/bin/bash

# Preempt Kubernetes Cleanup Script
# Similar to "docker-compose down"

set -e

echo "🗑️  Stopping and removing all Preempt resources..."

# Check if namespace exists
if kubectl get namespace preempt &> /dev/null; then
    echo "⏳ Deleting namespace 'preempt' (this will remove all resources)..."
    kubectl delete namespace preempt
    
    echo "⏳ Waiting for namespace to be fully deleted..."
    kubectl wait --for=delete namespace/preempt --timeout=60s || true
    
    echo "✅ All resources deleted!"
else
    echo "ℹ️  Namespace 'preempt' not found. Nothing to delete."
fi

echo ""
echo "🧹 Cleanup complete!"
echo ""
echo "Note: PersistentVolumes may be retained depending on reclaim policy."
echo "To start again: ./deploy.sh"
echo ""
