BINARY_NAME := aws-console
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
GIT_REF := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_VERSION ?= $(VERSION)-dev+$(GIT_REF)
LDFLAGS := -X github.com/eculver/aws-console/cmd.Version=$(BUILD_VERSION)

.DEFAULT_GOAL := build

.PHONY: build test test-release coverage coverage-html install clean fmt vet release release-major release-minor release-bugfix

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .

test:
	go test -v ./...

test-release:
	@./scripts/release_test.sh

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@if command -v open >/dev/null 2>&1; then open coverage.html || echo "coverage report generated at coverage.html"; \
	elif command -v xdg-open >/dev/null 2>&1; then xdg-open coverage.html || echo "coverage report generated at coverage.html"; \
	else echo "coverage report generated at coverage.html"; fi

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -f $(BINARY_NAME)

fmt:
	go fmt ./...

vet:
	go vet ./...

release:
	@if [ -z "$(ARGS)" ]; then \
		echo "Usage: make release ARGS='major|minor|bugfix -m \"Message\" [--push] [--yes]'"; \
		exit 1; \
	fi
	@./scripts/release.sh $(ARGS)

release-major:
	@if [ -z "$(MESSAGE)" ]; then \
		echo "Usage: make release-major MESSAGE='Message' [PUSH=1] [YES=1]"; \
		exit 1; \
	fi
	@./scripts/release.sh major -m "$(MESSAGE)" $(if $(PUSH),--push,) $(if $(YES),--yes,)

release-minor:
	@if [ -z "$(MESSAGE)" ]; then \
		echo "Usage: make release-minor MESSAGE='Message' [PUSH=1] [YES=1]"; \
		exit 1; \
	fi
	@./scripts/release.sh minor -m "$(MESSAGE)" $(if $(PUSH),--push,) $(if $(YES),--yes,)

release-bugfix:
	@if [ -z "$(MESSAGE)" ]; then \
		echo "Usage: make release-bugfix MESSAGE='Message' [PUSH=1] [YES=1]"; \
		exit 1; \
	fi
	@./scripts/release.sh bugfix -m "$(MESSAGE)" $(if $(PUSH),--push,) $(if $(YES),--yes,)
