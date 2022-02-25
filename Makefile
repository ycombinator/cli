include .bingo/Variables.mk

.PHONY: help
help:  ## This help dialog.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[$$()% 0-9a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


.PHONY: check
check: lint  ## CI code checks

xata:
	go build -o xata

.PHONY: lint
lint: $(GOLANGCI_LINT)
	echo "Linting source code..." 
	@$(GOLANGCI_LINT) run --timeout 2m --disable-all -e ST1005 -E forcetypeassert,goimports,goconst,gocritic,misspell,nolintlint,prealloc,staticcheck,unused,stylecheck,gosimple,govet,ineffassign,structcheck ./...

.PHONY: fmt
fmt: $(GOIMPORTS)   ## Format source code
	@gofmt -w -s .
	@$(GOIMPORTS) -w ./

.PHONY: test
test: xata   ## Run unit and e2e tests
	go test -race -failfast -v ./...

.PHONY: init
init:
	git config --local core.hooksPath .githooks

# install all tools used by the xata project
.PHONY: install-tools
install-tools:  ## Install development tools
	go get -d -modfile ./.bingo/bingo.mod github.com/bwplotka/bingo
	go install -modfile ./.bingo/bingo.mod github.com/bwplotka/bingo
	bingo get -l

.PHONY: generate
generate:       ## Run code generators
	go generate ./...
	$(MAKE) fmt

.PHONY: gomoddownload
gomoddownload:  ## Download all go modules.
	go mod download

.PHONY: dist
dist: $(GORELEASER)  ## Create a snapshot dist
	$(GORELEASER) release --snapshot --rm-dist