PKG = github.com/k1LoW/mo
COMMIT = $(shell git rev-parse --short HEAD)

BUILD_LDFLAGS = "-s -w -X $(PKG)/version.Revision=$(COMMIT)"

default: test

ci: depsdev generate test

generate:
	go generate ./internal/static/

test:
	cd internal/frontend && pnpm install && pnpm run test:coverage
	go test ./... -coverprofile=coverage.out -covermode=count -count=1

build: generate
	go build -ldflags=$(BUILD_LDFLAGS) -trimpath -o mo .

dev: generate
	go run . $(ARGS)

screenshot: build
	cd internal/frontend && pnpm run screenshots

lint:
	golangci-lint run ./...
	go vet -vettool=`which gostyle` -gostyle.config=$(PWD)/.gostyle.yml ./...

depsdev:
	go install github.com/Songmu/gocredits/cmd/gocredits@latest
	go install github.com/k1LoW/gostyle@latest

prerelease_for_tagpr: depsdev generate
	go mod download
	gocredits -w .
	git add CHANGELOG.md CREDITS go.mod go.sum

.PHONY: default ci generate test build dev screenshot lint depsdev prerelease_for_tagpr
