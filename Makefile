# Photo Backup Manager backend. Run `make help` for the list of targets.
#
# GO defaults to the snap binary because the `go` wrapper prints nothing inside sandboxed shells;
# override with `make GO=go ...` if your PATH has a normal Go install.
GO ?= $(shell command -v /snap/go/current/bin/go || command -v go)
TEST_DATABASE_URL ?= postgres:///photobackup_test?host=/var/run/postgresql

.PHONY: help run migrate rollback build test check fmt testdb phonesim live-check testdata

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'

run: ## Run the HTTP API (applies migrations first)
	$(GO) run ./cmd appserver

migrate: ## Apply database migrations and exit
	$(GO) run ./cmd migrate

rollback: ## Revert the newest N migrations: make rollback n=1
	$(GO) run ./cmd rollback $(n)

build: ## Build bin/server and bin/phonesim
	$(GO) build -o bin/server ./cmd
	$(GO) build -o bin/phonesim ./cmd/phonesim

test: ## Run all tests (DB tests use TEST_DATABASE_URL; packages run serially because they share it)
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' $(GO) test -count=1 -p 1 ./...

check: ## gofmt + vet + test: run before every commit
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "run: make fmt" && exit 1)
	$(GO) vet ./...
	$(MAKE) test

fmt: ## Format all Go code
	gofmt -w .

testdb: ## Create the disposable test database (once)
	createdb photobackup_test

phonesim: ## Run the stand-in phone against testdata/phone
	$(GO) run ./cmd/phonesim -dir testdata/phone

live-check: ## Exercise every API against REAL Google (server running, accounts connected, make build first)
	python3 -I scripts/live_check.py

testdata: ## Generate test photos: make testdata set=before|after
	python3 -I scripts/gen_testdata.py testdata/phone $(set)
