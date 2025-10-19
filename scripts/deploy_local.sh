#!/usr/bin/env bash
set -euo pipefail

APP_NAME="ticket-quality-service"
IMAGE_TAG="${APP_NAME}:dev"
PORT=50051

echo ""
echo "🚀 Deploying ${APP_NAME} locally with Minikube..."
echo ""

# use kubectl directly if available, otherwise use Minikube’s one
if ! command -v kubectl &> /dev/null; then
  if command -v minikube &> /dev/null; then
    echo "Using Minikube’s built-in kubectl..."
    kubectl() { minikube kubectl -- "$@"; }
  else
    echo "kubectl not found, and Minikube isn't installed either."
    exit 1
  fi
fi

# make sure kubectl is working
if ! kubectl version --client >/dev/null 2>&1; then
  echo "kubectl isn't responding properly. Check your setup."
  exit 1
fi

# switch Docker to Minikube’s environment
echo "→ Setting Docker environment for Minikube..."
eval "$(minikube docker-env)"

# build the image
echo "→ Building Docker image using Dockerfile.dev..."
docker build -t "${IMAGE_TAG}" -f Dockerfile.dev .

# deploy Redis if not running already
if ! kubectl get deployment redis >/dev/null 2>&1; then
  echo "→ Redis not found, deploying..."
  kubectl apply -f k8s/redis.yaml
  kubectl wait --for=condition=ready pod -l app=redis --timeout=90s || true
fi

echo "→ Setting up persistent storage (PV/PVC)..."
# Apply the PersistentVolume and PersistentVolumeClaim manifests
kubectl apply -f k8s/local/pv.yaml
kubectl apply -f k8s/local/pvc.yaml

echo "→ Copying database file into the Minikube volume..."
# Ensure the hostPath directory exists inside the Minikube node before copying
minikube ssh 'sudo mkdir -p /mnt/data/ticket-quality && sudo chmod 777 /mnt/data/ticket-quality'

# Copy the local database file to the hostPath directory on the Minikube node
minikube cp data/database.db /mnt/data/ticket-quality/database.db

# deploy the app
echo "→ Deploying ${APP_NAME}..."
kubectl delete deployment "${APP_NAME}" --ignore-not-found >/dev/null 2>&1 || true
kubectl delete service "${APP_NAME}" --ignore-not-found >/dev/null 2>&1 || true
kubectl apply -f k8s/local/service.yaml
kubectl apply -f k8s/local/deployment.yaml

echo "→ Waiting for pod(s) to be ready..."
TIMEOUT=120
SLEEP_INTERVAL=5
ELAPSED=0

while [ $ELAPSED -lt $TIMEOUT ]; do
  READY=$(kubectl get pods -l app="${APP_NAME}" --no-headers 2>/dev/null | grep -c "Running" || true)

  if [ "$READY" -ge 1 ]; then
    echo "✅ ${APP_NAME} is up (${READY} pod(s) running)"
    break
  fi

  echo "  ...still waiting (${ELAPSED}s)"
  kubectl get pods -l app="${APP_NAME}" --no-headers || true
  sleep $SLEEP_INTERVAL
  ELAPSED=$((ELAPSED + SLEEP_INTERVAL))
done

if [ "$READY" -lt 1 ]; then
  echo "❌ Pods didn't start within ${TIMEOUT}s"
  echo ""
  echo "Here’s the current pod state:"
  kubectl describe pods -l app="${APP_NAME}" | tail -n 20
  exit 1
fi

echo "→ Port-forwarding service on localhost:${PORT}..."
kubectl port-forward service/${APP_NAME} ${PORT}:${PORT} &
FORWARD_PID=$!

trap 'echo "Stopping port-forward..."; kill ${FORWARD_PID} 2>/dev/null' EXIT

sleep 2
echo ""
echo "Service is up and reachable at localhost:${PORT}"
echo ""
echo "🧪 Test the v1 API with grpcurl:"
echo ""
echo "List services:"
echo "  grpcurl -plaintext localhost:${PORT} list"
echo ""
echo "Get overall quality score:"
echo "  grpcurl -plaintext \\"
echo "    -d '{\"start_date\": \"2019-01-01T00:00:00Z\", \"end_date\": \"2019-12-31T00:00:00Z\"}' \\"
echo "    localhost:${PORT} ticketscoring.v1.TicketScoring/GetOverallQualityScore"
echo ""
echo "Get scores by ticket:"
echo "  grpcurl -plaintext \\"
echo "    -d '{\"start_date\": \"2019-01-01T00:00:00Z\", \"end_date\": \"2019-12-31T00:00:00Z\"}' \\"
echo "    localhost:${PORT} ticketscoring.v1.TicketScoring/GetScoresByTicket"
echo ""

wait ${FORWARD_PID}
