package api

const openapiSpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Illithid DHCP Server API",
    "description": "REST API for the Illithid security-research DHCP server. Provides lease management, interception rules for traffic redirection, and connected client visibility.",
    "version": "2.0.0",
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
                }
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
            "description": "MAC address (colon-separated, e.g. aa:bb:cc:dd:ee:01)",
            "schema": {
              "type": "string",
              "pattern": "^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$"
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
                }
              }
            }
          }
        }
      }
    },
    "/intercepts": {
      "get": {
        "summary": "List all interception leases",
        "description": "Returns all configured interception rules. These rules override normal DHCP responses for specific MAC addresses, redirecting traffic through a custom gateway.",
        "operationId": "listIntercepts",
        "responses": {
          "200": {
            "description": "List of interception leases (empty array if none)",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/InterceptLease"
                  }
                }
              }
            }
          }
        }
      },
      "post": {
        "summary": "Create an interception lease",
        "description": "Creates a new interception rule for a specific MAC address. When the DHCP server receives a request from this MAC on the specified interface, it will respond with the custom IP, gateway, and DNS instead of the normal pool allocation.",
        "operationId": "createIntercept",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/InterceptLeaseRequest"
              },
              "example": {
                "mac": "aa:bb:cc:dd:ee:01",
                "ip": "192.168.1.50",
                "gateway": "192.168.1.254",
                "dns": ["10.0.0.53"],
                "interface": "eth0"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Interception lease created",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/InterceptLease"
                }
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          },
          "409": {
            "description": "Interception lease already exists for this MAC",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          }
        }
      }
    },
    "/intercepts/{mac}": {
      "put": {
        "summary": "Update an interception lease",
        "description": "Updates the interception rule for the given MAC address. The original creation timestamp is preserved.",
        "operationId": "updateIntercept",
        "parameters": [
          {
            "name": "mac",
            "in": "path",
            "required": true,
            "description": "MAC address (colon-separated)",
            "schema": {
              "type": "string",
              "pattern": "^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$"
            }
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/InterceptLeaseUpdateRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Interception lease updated",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/InterceptLease"
                }
              }
            }
          },
          "400": {
            "description": "Validation error",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          },
          "404": {
            "description": "No interception lease found for this MAC",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          }
        }
      },
      "delete": {
        "summary": "Delete an interception lease",
        "description": "Removes the interception rule for the given MAC address. Any active tracking lease for this client is also removed. The client will receive normal DHCP responses on their next renewal.",
        "operationId": "deleteIntercept",
        "parameters": [
          {
            "name": "mac",
            "in": "path",
            "required": true,
            "description": "MAC address (colon-separated)",
            "schema": {
              "type": "string",
              "pattern": "^([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}$"
            }
          }
        ],
        "responses": {
          "204": {
            "description": "Interception lease deleted"
          },
          "404": {
            "description": "No interception lease found for this MAC",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Error"
                }
              }
            }
          }
        }
      }
    },
    "/clients": {
      "get": {
        "summary": "List connected clients",
        "description": "Returns all clients that have completed a DHCP handshake. Each entry includes an 'intercepted' flag indicating whether the client is being served by an interception rule.",
        "operationId": "listClients",
        "responses": {
          "200": {
            "description": "List of connected clients (empty array if none)",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "$ref": "#/components/schemas/Lease"
                  }
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
          "intercepted": {
            "type": "boolean",
            "description": "Whether this client is being served by an interception rule",
            "example": false
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
        "required": ["mac", "ip", "interface", "intercepted", "expires_at", "created_at"]
      },
      "InterceptLease": {
        "type": "object",
        "properties": {
          "mac": {
            "type": "string",
            "description": "Client MAC address to intercept",
            "example": "aa:bb:cc:dd:ee:01"
          },
          "ip": {
            "type": "string",
            "format": "ipv4",
            "description": "IP address to assign to the intercepted client",
            "example": "192.168.1.50"
          },
          "gateway": {
            "type": "string",
            "format": "ipv4",
            "description": "Custom gateway for traffic inspection",
            "example": "192.168.1.254"
          },
          "dns": {
            "type": "array",
            "items": {
              "type": "string",
              "format": "ipv4"
            },
            "description": "Custom DNS server(s)",
            "example": ["10.0.0.53"]
          },
          "interface": {
            "type": "string",
            "description": "Network interface this rule applies to",
            "example": "eth0"
          },
          "created_at": {
            "type": "string",
            "format": "date-time",
            "description": "Rule creation timestamp"
          }
        },
        "required": ["mac", "ip", "gateway", "dns", "interface", "created_at"]
      },
      "InterceptLeaseRequest": {
        "type": "object",
        "properties": {
          "mac": {
            "type": "string",
            "description": "Client MAC address to intercept",
            "example": "aa:bb:cc:dd:ee:01"
          },
          "ip": {
            "type": "string",
            "format": "ipv4",
            "description": "IP address to assign",
            "example": "192.168.1.50"
          },
          "gateway": {
            "type": "string",
            "format": "ipv4",
            "description": "Custom gateway",
            "example": "192.168.1.254"
          },
          "dns": {
            "type": "array",
            "items": {
              "type": "string",
              "format": "ipv4"
            },
            "description": "Custom DNS server(s)",
            "example": ["10.0.0.53"]
          },
          "interface": {
            "type": "string",
            "description": "Target network interface",
            "example": "eth0"
          }
        },
        "required": ["mac", "ip", "gateway", "dns", "interface"]
      },
      "InterceptLeaseUpdateRequest": {
        "type": "object",
        "properties": {
          "ip": {
            "type": "string",
            "format": "ipv4",
            "description": "IP address to assign",
            "example": "192.168.1.60"
          },
          "gateway": {
            "type": "string",
            "format": "ipv4",
            "description": "Custom gateway",
            "example": "192.168.1.254"
          },
          "dns": {
            "type": "array",
            "items": {
              "type": "string",
              "format": "ipv4"
            },
            "description": "Custom DNS server(s)",
            "example": ["10.0.0.53"]
          },
          "interface": {
            "type": "string",
            "description": "Target network interface",
            "example": "eth0"
          }
        },
        "required": ["ip", "gateway", "dns", "interface"]
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
