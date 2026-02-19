BINARY_NAME := aws-console

.DEFAULT_GOAL := build

.PHONY: build test coverage coverage-html install clean fmt vet

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v ./...

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
	go install .

clean:
	rm -f $(BINARY_NAME)

fmt:
	go fmt ./...

vet:
	go vet ./...
