run-example-basic:
	go run examples/basic/main.go

run-example-webhook-server:
	go run examples/webhook_server/main.go

run-example-zts:
	go run ./examples/zts/ -config examples/zts/config.yaml

# Monitoring — Prometheus + Grafana stack
monitoring-up:
	docker compose -f examples/monitoring/docker-compose.yml up -d

monitoring-down:
	docker compose -f examples/monitoring/docker-compose.yml down

monitoring-restart:
	docker compose -f examples/monitoring/docker-compose.yml down
	docker compose -f examples/monitoring/docker-compose.yml up -d

monitoring-clean:
	docker compose -f examples/monitoring/docker-compose.yml down -v

# Tests
test:
	go test ./... -count=1
