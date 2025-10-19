.PHONY: deploy
deploy:
	@echo "🚀 Running local deployment script..."
	@chmod +x scripts/deploy_local.sh
	@./scripts/deploy_local.sh

.PHONY: test
test:
	@echo "🧪 Running test script..."
	@chmod +x scripts/test.sh
	@./scripts/test.sh

.PHONY: test-all
test-all:
	@echo "🧪 Running test script with E2E tests..."
	@chmod +x scripts/test.sh
	@./scripts/test.sh --e2e

.PHONY: build
build: proto
	@echo "🏗️  Building with Docker Compose..."
	@docker compose build

.PHONY: run
run: proto
	@echo "🚀 Running application with Docker Compose..."
	@docker compose up --build

.PHONY: run-detached
run-detached: proto
	@echo "🚀 Running application with Docker Compose (detached)..."
	@docker compose up --build -d
	@echo ""
	@echo "✅ Service running at localhost:50051"
	@echo ""
	@echo "🧪 Test with grpcurl:"
	@echo "  grpcurl -plaintext localhost:50051 list"
	@echo "  grpcurl -plaintext \\"
	@echo "    -d '{\"start_date\": \"2019-01-01T00:00:00Z\", \"end_date\": \"2019-12-31T00:00:00Z\"}' \\"
	@echo "    localhost:50051 ticketscoring.v1.TicketScoring/GetOverallQualityScore"

.PHONY: stop
stop:
	@echo "🛑 Stopping Docker Compose services..."
	@docker compose down

.PHONY: proto
proto:
	@echo "🔧 Generating protobuf files..."
	@protoc --go_out=. --go_opt=paths=source_relative \
	        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	        api/v1/ticketscoring.proto
	@echo "✅ Protobuf files generated successfully"

.PHONY: help
help:
	@echo ""
	@echo "🧠 QA Server Makefile Commands"
	@echo "-----------------------------------------------"
	@echo " make deploy           - Run local deployment script (Kubernetes)"
	@echo " make run              - Run application with Docker Compose"
	@echo " make run-detached     - Run application with Docker Compose (background)"
	@echo " make stop             - Stop Docker Compose services"
	@echo " make build            - Build with Docker Compose"
	@echo " make test             - Run test script (unit tests + benchmarks)"
	@echo " make test-all         - Run test script with E2E tests included"
	@echo " make proto            - Generate protobuf files from .proto definitions"
	@echo " make help             - Show this help message"
	@echo ""
