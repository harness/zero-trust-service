# examples/monitoring

A **local Prometheus + Grafana stack** for monitoring a running ZTS instance. Launches both services via Docker Compose, auto-provisions Prometheus as a Grafana datasource, and loads a pre-built ZTS overview dashboard — so you get metrics visibility immediately after starting the stack.

## Folder Structure

```
monitoring/
├── docker-compose.yml                          Prometheus + Grafana services
├── prometheus.yml                              Prometheus scrape config (targets ZTS admin port)
└── grafana/
    └── provisioning/
        ├── datasources/
        │   └── prometheus.yml                  Auto-provisions Prometheus as a datasource
        └── dashboards/
            ├── provider.yml                    Dashboard provisioning config
            └── json/
                └── zts-overview.json           Pre-built ZTS overview dashboard
```

## How to Use

From the repository root:

```bash
make monitoring-up       # start Prometheus + Grafana
make monitoring-down     # stop the stack
make monitoring-clean    # stop and remove volumes
```

Or from this directory:

```bash
docker compose up -d     # start
docker compose down      # stop
docker compose down -v   # stop and remove volumes
```

### Access

| Service | URL | Credentials |
|---------|-----|-------------|
| Prometheus | http://localhost:9091 | — |
| Grafana | http://localhost:3002 | Auto-login (anonymous admin) |

### Scrape Target

Prometheus is configured to scrape `host.docker.internal:8898` (the ZTS admin server's default port) every 5 seconds. If your ZTS admin port is different, edit `prometheus.yml`.

### Grafana

Grafana starts with anonymous admin access (no login required). The Prometheus datasource and the ZTS overview dashboard are auto-provisioned on startup — no manual setup needed.
