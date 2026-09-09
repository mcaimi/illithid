package api

const openapiSpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Illithid DHCP Server API",
    "description": "REST API for the Illithid security-research DHCP server. Provides lease management for DHCP allocations across configured network interfaces.",
    "version": "1.0.0",
    "license": {
      "name": "MIT"
    }
  },
  "servers": [
    {
      "url": "/api/v1",
      "description": "API v1"
    }
  ],
  "paths": {
    "/leases": {
      "get": {
        "summary": "List all DHCP leases",
        "description": "Returns all active DHCP leases across all configured interfaces.",
        "operationId": "listLeases",
        "responses": {
          "200": {
            "description": "List of leases (empty array if none)",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Lease"
                  }
                },
                "example": [
                  {
                    "mac": "aa:bb:cc:dd:ee:01",
                    "ip": "192.168.1.100",
                    "hostname": "workstation-1",
                    "interface": "eth0",
                    "expires_at": "2026-09-09T16:00:00Z",
                    "created_at": "2026-09-09T15:00:00Z"
                  }
                ]
              }
            }
          }
        }
      }
    },
    "/leases/{mac}": {
      "delete": {
        "summary": "Delete a DHCP lease",
        "description": "Removes the lease for the given MAC address and releases the IP back to its interface pool.",
        "operationId": "deleteLease",
        "parameters": [
          {
            "name": "mac",
            "in": "path",
            "required": true,
            "description": "MAC address of the lease to delete (colon-separated, e.g. aa:bb:cc:dd:ee:01)",
            "schema": {
              "type": "string",
              "pattern": "^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$",
              "example": "aa:bb:cc:dd:ee:01"
            }
          }
        ],
        "responses": {
          "204": {
            "description": "Lease deleted and IP released"
          },
          "404": {
            "description": "No lease found for this MAC address",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                },
                "example": {
                  "error": "lease not found"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Lease": {
        "type": "object",
        "properties": {
          "mac": {
            "type": "string",
            "description": "Client MAC address (lowercase, colon-separated)",
            "example": "aa:bb:cc:dd:ee:01"
          },
          "ip": {
            "type": "string",
            "format": "ipv4",
            "description": "Assigned IPv4 address",
            "example": "192.168.1.100"
          },
          "hostname": {
            "type": "string",
            "description": "Client hostname from DHCP option 12 (may be empty)",
            "example": "workstation-1"
          },
          "interface": {
            "type": "string",
            "description": "Network interface that served this lease",
            "example": "eth0"
          },
          "expires_at": {
            "type": "string",
            "format": "date-time",
            "description": "Lease expiration timestamp"
          },
          "created_at": {
            "type": "string",
            "format": "date-time",
            "description": "Lease creation timestamp"
          }
        },
        "required": ["mac", "ip", "interface", "expires_at", "created_at"]
      },
      "Error": {
        "type": "object",
        "properties": {
          "error": {
            "type": "string",
            "description": "Error message"
          }
        },
        "required": ["error"]
      }
    }
  }
}`
