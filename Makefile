BINARY   := illithid
GOFLAGS  := -trimpath
LDFLAGS  := -s -w

.PHONY: build cli all run clean test vet

build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/illithid

cli:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/illithidctl ./cmd/illithidctl

all: build cli

run: build
	sudo bin/$(BINARY) -config parameters.yaml

clean:
	rm -rf bin/

test:
	go test -race -count=1 ./...

vet:
	go vet ./...
