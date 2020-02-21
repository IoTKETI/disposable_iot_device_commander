.PHONY: build clean

GO=CGO_ENABLED=0 GO111MODULE=on go

MICROSERVICES=cmd/device-simple/device-commander
.PHONY: $(MICROSERVICES)

VERSION=$(shell cat ./VERSION)

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-simple.Version=$(VERSION)"

GIT_SHA=$(shell git rev-parse HEAD)

build: $(MICROSERVICES)
	$(GO) install -tags=safe

cmd/device-simple/device-commander:
	$(GO) build $(GOFLAGS) -o ./cmd/device-simple/device-commander ./cmd/device-simple

clean:
	rm -f $(MICROSERVICES)
