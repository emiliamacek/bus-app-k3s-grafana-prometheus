# Bus App Observability Sandbox

A local Kubernetes observability sandbox built with **Go**, **Docker**, **k3s**, **Prometheus**, and **Grafana**.

The project runs a simple bus tracking API inside a local k3s cluster and exposes custom Prometheus metrics. It also deploys a monitoring stack with Grafana, Prometheus, Alertmanager, kube-state-metrics, and node-exporter.

---

## What this project does

This project creates a local environment for testing Kubernetes deployments and observability.

It includes:

* a Go HTTP API simulating a bus tracking backend,
* Prometheus metrics exposed at `/metrics`,
* a local Docker registry,
* a local k3s Kubernetes cluster,
* Kubernetes manifests for the application,
* a monitoring stack installed through k3s HelmChart manifests,
* Grafana and Prometheus exposed locally through NodePort services.

---

## Architecture

```text
Local machine
│
├── Docker Compose
│   ├── local registry :5000
│   ├── manifests-init
│   └── k3s cluster
│
├── k3s cluster
│   ├── bus-app Deployment
│   ├── bus-app Service
│   ├── ServiceMonitor
│   ├── PrometheusRule
│   └── kube-prometheus-stack
│       ├── Prometheus
│       ├── Grafana
│       ├── Alertmanager
│       ├── kube-state-metrics
│       └── node-exporter
│
└── Browser
    ├── Bus app:    http://localhost:8080
    ├── Grafana:    http://localhost:3000
    └── Prometheus: http://localhost:9090
```

---

## Application

The application is a simple Go backend simulating a bus tracking API.

It exposes endpoints for:

* listing buses,
* getting route information,
* simulating GPS updates,
* subscribing to bus stop updates,
* exposing Prometheus metrics.

The app listens on port `8080`.

---

## API endpoints

### `GET /buses`

Returns a list of simulated buses.

Example response:

```json
[
  {
    "id": "bus-001",
    "line": "194",
    "latitude": 52.2297,
    "longitude": 21.0122,
    "speed_kmh": 42.5
  }
]
```

---

### `GET /route/{id}`

Returns simulated route information for a route ID.

This endpoint intentionally adds random latency to make duration metrics more interesting.

Example:

```bash
curl http://localhost:8080/route/194
```

Example response:

```json
{
  "route_id": "194",
  "stops": 12,
  "estimated_minutes": 25
}
```

---

### `POST /buses/{id}/position`

Simulates a GPS position update for a bus.

This endpoint randomly returns errors to generate HTTP 500 responses for observability testing.

Example:

```bash
curl -X POST http://localhost:8080/buses/bus-001/position
```

Example success response:

```json
{
  "status": "ok",
  "bus_id": "bus-001"
}
```

Example error response:

```json
{
  "error": "GPS module timeout"
}
```

---

### `GET /subscribe/{stop_id}`

Simulates subscribing to updates for a bus stop.

This endpoint increments a custom gauge metric that intentionally never decreases, which can be used to demonstrate memory-leak-like behavior or suspicious growth.

Example:

```bash
curl http://localhost:8080/subscribe/stop-123
```

Example response:

```json
{
  "stop_id": "stop-123",
  "status": "subscribed"
}
```

---

### `GET /metrics`

Exposes Prometheus metrics.

Example:

```bash
curl http://localhost:8080/metrics
```

---

## Custom Prometheus metrics

The application exposes custom metrics using the Prometheus Go client.

### `http_requests_total`

Counter incremented for every HTTP request.

Labels:

* `method`
* `path`
* `status`

Useful for calculating:

* request rate,
* error rate,
* traffic per endpoint.

Example PromQL:

```promql
rate(http_requests_total[5m])
```

---

### `http_request_duration_seconds`

Histogram measuring HTTP request duration.

Labels:

* `method`
* `path`

Useful for latency queries and percentiles.

Example PromQL for p99 latency:

```promql
histogram_quantile(
  0.99,
  rate(http_request_duration_seconds_bucket[5m])
)
```

---

### `http_active_requests`

Gauge showing the current number of in-flight HTTP requests.

Useful for observing active load on the application.

---

### `bus_stop_subscribers_total`

Gauge incremented by the `/subscribe/{stop_id}` endpoint.

This metric intentionally never decreases, so it can be used to simulate suspicious growth or leak-like behavior.

---

## Kubernetes manifests

The `manifests/` directory contains Kubernetes resources automatically loaded by k3s.

### `manifests/bus-app.yaml`

Configures the bus application.

It includes:

* `Deployment` for `bus-app`,
* `Service` exposed through NodePort,
* `ServiceMonitor` for Prometheus scraping,
* `PrometheusRule` for alerting rules.

The NodePort service exposes the bus app locally on:

```text
http://localhost:8080
```

---

### `manifests/monitoring.yml`

Configures the monitoring stack through k3s HelmChart resources.

It installs and configures:

* Prometheus,
* Grafana,
* Alertmanager,
* kube-state-metrics,
* node-exporter.

Grafana and Prometheus are exposed locally through NodePort services:

```text
Grafana:    http://localhost:3000
Prometheus: http://localhost:9090
```

Default Grafana credentials:

```text
username: admin
password: admin
```

---

## Docker Compose setup

The `docker-compose.yml` file starts the local infrastructure.

It defines three services:

### `registry`

Runs a local Docker registry on port `5000`.

This is used to store the locally built `bus-app` Docker image.

```text
localhost:5000/bus-app:latest
```

---

### `manifests-init`

Copies Kubernetes manifests from the local `manifests/` directory into the shared k3s manifests volume.

This allows k3s to automatically apply the manifests when the cluster starts.

---

### `k3s`

Runs a local k3s server in Docker.

It:

* starts the Kubernetes cluster,
* disables Traefik,
* exposes the Kubernetes API on port `6443`,
* mounts the manifests volume,
* uses `registries.yaml` to connect to the local Docker registry.

Exposed ports:

| Local port | Cluster port | Service        |
| ---------: | -----------: | -------------- |
|     `6443` |       `6443` | Kubernetes API |
|     `3000` |      `30000` | Grafana        |
|     `9090` |      `30090` | Prometheus     |
|     `8080` |      `30080` | Bus app        |

---

## Local registry configuration

The `registries.yaml` file configures k3s to use the local Docker registry.

This allows Kubernetes to pull the image:

```text
localhost:5000/bus-app:latest
```

from the registry container.

---

## Dockerfile

The `Dockerfile` builds the Go bus application and its dependencies into a container image.

The resulting image is tagged and pushed to the local registry before k3s starts the application.

---

## Setup script

The `setup.sh` script automates the local setup.

It performs the following steps:

1. starts the local registry,
2. builds the `bus-app` Docker image,
3. pushes the image to the local registry,
4. starts the k3s cluster,
5. prints useful service URLs.

Main commands performed by the script:

```bash
docker compose up -d registry
docker build -t localhost:5000/bus-app:latest .
docker push localhost:5000/bus-app:latest
docker compose up -d k3s
```

---

## Running the project

Start the environment:

```bash
./setup.sh
```

Or, if using the Makefile:

```bash
make setup
```

After startup, wait a few minutes for k3s, Helm charts, Prometheus, and Grafana to become ready.

The setup script prints:

```text
Services will be ready in ~3-5 minutes.

Grafana:    http://localhost:3000  (admin / admin)
Prometheus: http://localhost:9090
Bus app:    http://localhost:8080
```

---

## Accessing services

### Bus app

```text
http://localhost:8080
```

Example:

```bash
curl http://localhost:8080/buses
```

---

### Prometheus

```text
http://localhost:9090
```

Useful queries:

```promql
rate(http_requests_total[5m])
```

```promql
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
```

```promql
http_active_requests
```

```promql
bus_stop_subscribers_total
```

---

### Grafana

```text
http://localhost:3000
```

Default credentials:

```text
admin / admin
```

---

## Entering the k3s container

To open a shell inside the k3s container:

```bash
docker exec -it k3s-cluster sh
```

From there, you can inspect Kubernetes resources:

```bash
kubectl get pods -A
kubectl get svc -A
kubectl get servicemonitor -A
kubectl get prometheusrule -A
```

---

## Useful Kubernetes commands

If your kubeconfig is available locally, you can run:

```bash
kubectl get nodes
kubectl get pods -A
kubectl get svc -A
```

To inspect the bus app:

```bash
kubectl get deployment bus-app
kubectl get service bus-app
kubectl describe deployment bus-app
kubectl logs -l app=bus-app
```

To inspect monitoring resources:

```bash
kubectl get pods -n monitoring
kubectl get servicemonitor -A
kubectl get prometheusrule -A
```

---

## Stopping the environment

Stop the containers:

```bash
docker compose down
```

If you also want to remove volumes:

```bash
docker compose down -v
```

This removes the k3s manifests volume as well.

---

## Notes

This project is intended for local learning and experimentation.

It demonstrates:

* containerizing a Go application,
* running a local Docker registry,
* running k3s through Docker Compose,
* deploying Kubernetes manifests automatically,
* exposing services through NodePort,
* collecting custom Prometheus metrics,
* using ServiceMonitor for scraping,
* defining PrometheusRule alerts,
* visualizing metrics in Grafana.

It is not intended as a production-ready Kubernetes setup.
