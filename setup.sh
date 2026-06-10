#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

echo "Starting local registry..."
docker compose up -d registry

echo "Building bus-app image..."
docker build -t localhost:5000/bus-app:latest .

echo "Pushing to local registry..."
docker push localhost:5000/bus-app:latest

echo "Starting k3s..."
docker compose up -d k3s

echo ""
echo "Services will be ready in ~3-5 minutes."
echo "  Grafana:    http://localhost:3000  (admin / admin)"
echo "  Prometheus: http://localhost:9090"
echo "  Bus app:    http://localhost:8080"
echo ""
echo "If you want to use docker shell, enter: "
echo "docker exec -it k3s-cluster sh"
