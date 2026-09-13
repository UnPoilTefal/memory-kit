BINARY  := memctl
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

help: ## Afficher cette aide
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Compiler memctl dans ./bin
	@mkdir -p bin && go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/memctl

install: ## Installer memctl dans le GOPATH
	go install -ldflags "$(LDFLAGS)" ./cmd/memctl

test: ## Lancer les tests
	go test -race ./...

vet: ## Analyse statique
	go vet ./...
	golangci-lint run

dogfood: build ## Appliquer le kit a son propre corpus et a son registre d'exemple
	./bin/$(BINARY) lint examples/corpus --strict
	./bin/$(BINARY) perimeter examples/perimeter.yml
	./bin/$(BINARY) gate examples/readiness/prete.yml --corpus examples/corpus --no-record

verify: build ## Rejouer les preuves du corpus d'exemple
	./bin/$(BINARY) verify examples/corpus --allow-exec

check: vet test dogfood ## Tout ce que la CI verifie

snapshot: ## Construire une release locale sans rien publier
	go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish,docker

release-check: ## Valider la configuration GoReleaser
	go run github.com/goreleaser/goreleaser/v2@latest check

clean: ## Supprimer les artefacts de build
	rm -rf bin dist

.PHONY: help build install test vet dogfood verify check snapshot release-check clean
