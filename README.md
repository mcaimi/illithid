# Illithid

A security-research DHCP server written in Go. Illithid serves DHCP leases across multiple network interfaces and can selectively redirect individual hosts to a custom gateway for traffic inspection.

## Features

- **Multi-interface DHCP server** — binds to specific network interfaces with independent IP pools, DNS, and gateway settings
- **In-memory lease management** — no on-disk persistence; IP pools, leases, and interception rules are kept entirely in memory
- **Traffic interception** — redirect any client (by MAC address) to a custom gateway, DNS, and IP without affecting other clients on the network
- **REST API** — full CRUD for leases and interception rules, plus a connected-clients view
- **Interactive API docs** — Swagger UI at `/api/docs`, OpenAPI 3.0 spec at `/api/v1/openapi.json`

## Requirements

- Go 1.21+
- Root privileges (DHCP binds to UDP port 67)

## Building

```sh
make build        # produces bin/illithid
```

Or directly:

```sh
go build -trimpath -ldflags '-s -w' -o bin/illithid ./cmd/illithid
```

## Configuration

Copy the example and edit it for your environment:

```sh
cp parameters.yaml.example parameters.yaml
```

### parameters.yaml

```yaml
interfaces:
  - name: "eth0"                    # network interface to listen on
    server_ip: "192.168.1.1"        # DHCP server identifier (your IP on this interface)
    dhcp:
      range_start: "192.168.1.100"  # first IP in the DHCP pool
      range_end: "192.168.1.200"    # last IP in the DHCP pool
      subnet_mask: "255.255.255.0"
      lease_duration: 3600          # seconds
    dns:
      - "8.8.8.8"
      - "8.8.4.4"
    gateway: "192.168.1.1"

api:
  listen: ":8080"                   # REST API bind address
```

You can configure multiple interfaces — each gets its own DHCP server instance and IP pool. The `server_ip` is explicitly set (not auto-detected) so you can control what server identity is advertised.

## Running

```sh
sudo bin/illithid -config parameters.yaml
```

The `-config` flag defaults to `parameters.yaml` in the current directory.

Use `make run` as a shortcut (runs `sudo` automatically).

Stop with `Ctrl-C` — the server shuts down gracefully, draining in-flight API requests.

## Container

A `Containerfile` is provided in `containers/` for running Illithid in a container. It uses a multi-stage build with a Debian Bookworm runtime image.

### Building the container image

```sh
podman build -f containers/Containerfile -t illithid .
```

Or with Docker:

```sh
docker build -f containers/Containerfile -t illithid .
```

### Running the container

The container expects a configuration file at `/etc/illithid/parameters.yaml`. Mount your local config file into the container:

```sh
podman run --net=host \
  -v ./parameters.yaml:/etc/illithid/parameters.yaml:Z \
  illithid
```

`--net=host` is required because the DHCP server needs direct access to the host's network interfaces and UDP port 67. Without it, the server cannot bind to specific interfaces or receive DHCP broadcast traffic.

To use a different config path or override the default command:

```sh
podman run --net=host \
  -v /path/to/my-config.yaml:/etc/illithid/parameters.yaml:Z \
  illithid
```

To run in the background:

```sh
podman run -d --name illithid --net=host \
  -v ./parameters.yaml:/etc/illithid/parameters.yaml:Z \
  illithid
```

### Exposed ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 67 | UDP | DHCP server |
| 8080 | TCP | REST API |

Note: when using `--net=host`, `EXPOSE` declarations are informational only — the container shares the host's network stack directly.

## REST API

All endpoints return JSON with `Content-Type: application/json`. The interactive Swagger UI is available at `http://localhost:8080/api/docs`.

### Leases

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/leases` | List all DHCP leases |
| `DELETE` | `/api/v1/leases/{mac}` | Delete a lease and release the IP back to the pool |

**Example — list leases:**

```sh
curl http://localhost:8080/api/v1/leases
```

```json
[
  {
    "mac": "aa:bb:cc:dd:ee:01",
    "ip": "192.168.1.100",
    "hostname": "workstation-1",
    "interface": "eth0",
    "intercepted": false,
    "expires_at": "2026-09-09T16:00:00Z",
    "created_at": "2026-09-09T15:00:00Z"
  }
]
```

**Example — delete a lease:**

```sh
curl -X DELETE http://localhost:8080/api/v1/leases/aa:bb:cc:dd:ee:01
# 204 No Content
```

### Interception

Interception leases override normal DHCP responses for a specific MAC address, redirecting the client to a custom IP, gateway, and DNS. The interception IP is user-specified and does not come from the DHCP pool.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/intercepts` | List all interception rules |
| `POST` | `/api/v1/intercepts` | Create an interception rule |
| `PUT` | `/api/v1/intercepts/{mac}` | Update an interception rule |
| `DELETE` | `/api/v1/intercepts/{mac}` | Delete an interception rule |

**Example — create an interception rule:**

```sh
curl -X POST http://localhost:8080/api/v1/intercepts \
  -H 'Content-Type: application/json' \
  -d '{
    "mac": "aa:bb:cc:dd:ee:01",
    "ip": "192.168.1.50",
    "gateway": "192.168.1.254",
    "dns": ["10.0.0.53"],
    "interface": "eth0"
  }'
```

```json
{
  "mac": "aa:bb:cc:dd:ee:01",
  "ip": "192.168.1.50",
  "gateway": "192.168.1.254",
  "dns": ["10.0.0.53"],
  "interface": "eth0",
  "created_at": "2026-09-09T15:00:00Z"
}
```

On the next DHCP request from `aa:bb:cc:dd:ee:01` on `eth0`, the server will respond with IP `192.168.1.50`, gateway `192.168.1.254`, and DNS `10.0.0.53` instead of the normal pool allocation.

**Example — update an interception rule:**

```sh
curl -X PUT http://localhost:8080/api/v1/intercepts/aa:bb:cc:dd:ee:01 \
  -H 'Content-Type: application/json' \
  -d '{
    "ip": "192.168.1.60",
    "gateway": "192.168.1.254",
    "dns": ["10.0.0.53"],
    "interface": "eth0"
  }'
```

**Example — remove an interception rule:**

```sh
curl -X DELETE http://localhost:8080/api/v1/intercepts/aa:bb:cc:dd:ee:01
# 204 No Content — client returns to normal DHCP on next renewal
```

### Connected Clients

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/clients` | List all clients that completed a DHCP handshake |

Each client entry includes an `intercepted` boolean indicating whether the client is being served by an interception rule.

```sh
curl http://localhost:8080/api/v1/clients
```

### API Documentation

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/docs` | Interactive Swagger UI |
| `GET` | `/api/v1/openapi.json` | OpenAPI 3.0 spec (JSON) |

## Architecture

```
cmd/illithid/main.go          Entrypoint, config loading, server wiring, signal handling
internal/
  config/config.go            YAML configuration types and validated loader
  pool/pool.go                Thread-safe in-memory IP pool (per interface)
  lease/store.go              Thread-safe in-memory lease store (shared across interfaces)
  intercept/store.go          Thread-safe in-memory interception rule store
  dhcp/handler.go             DHCP handler — DISCOVER/OFFER, REQUEST/ACK/NAK, RELEASE
  api/api.go                  Chi REST API — leases, intercepts, clients
  api/openapi.go              Embedded OpenAPI 3.0 specification
  api/docs.go                 Embedded Swagger UI HTML page
```

### DHCP Flow

```
Client DISCOVER
  |
  v
Handler.ServeDHCP
  |
  +-- intercepts.Lookup(mac) match? --> sendInterceptOffer (custom IP/GW/DNS)
  |
  +-- pool.Allocate(mac)            --> send normal OFFER (pool IP, default GW/DNS)

Client REQUEST
  |
  v
Handler.ServeDHCP
  |
  +-- intercepts.Lookup(mac) match? --> validate IP, record lease (intercepted=true), ACK
  |
  +-- pool.Lookup(mac)              --> validate IP, record lease, ACK
  |
  +-- mismatch                      --> NAK
```

### Concurrency

| Resource | Lock Type | Scope |
|----------|-----------|-------|
| `pool.IPPool` | `sync.Mutex` | One per interface, low contention |
| `lease.Store` | `sync.RWMutex` | Shared, read-heavy from API |
| `intercept.Store` | `sync.RWMutex` | Shared, read-heavy from DHCP handlers |

### Dependencies

| Library | Purpose |
|---------|---------|
| [insomniacslk/dhcp](https://github.com/insomniacslk/dhcp) | DHCPv4 packet handling and per-interface server |
| [go-chi/chi](https://github.com/go-chi/chi) | HTTP router (net/http compatible) |
| [go-yaml/yaml](https://github.com/go-yaml/yaml) | YAML configuration parsing |

## Testing

```sh
make test         # runs go test -race -count=1 ./...
make vet          # runs go vet ./...
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Compile to `bin/illithid` |
| `run` | Build and run with sudo |
| `test` | Run tests with race detector |
| `vet` | Static analysis |
| `clean` | Remove build artifacts |

## License

GPL v3 (see LICENSE file)
