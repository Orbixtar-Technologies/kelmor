.PHONY: bootstrap generate lint test test-integration test-security test-provisioning build package release dev portals

GO ?= go
API_PORT ?= 18080
SERVER_PORTAL_PORT ?= 18443
ACCOUNT_PORTAL_PORT ?= 18444

bootstrap:
	$(GO) mod download
	cd portals/server && npm install
	cd portals/account && npm install

generate:
	$(GO) generate ./...

lint:
	$(GO) fmt ./...
	$(GO) vet ./...

test:
	$(GO) test ./...

test-integration:
	$(GO) test ./tests/acceptance/... ./tests/provisioning/...

test-security:
	$(GO) test ./tests/security/... ./internal/filesystem/... ./agent/policy/...

test-provisioning:
	$(GO) test ./tests/provisioning/... ./internal/provisioning/...

build:
	mkdir -p dist/bin
	$(GO) build -o dist/bin/panel-api ./cmd/panel-api
	$(GO) build -o dist/bin/panel-worker ./cmd/panel-worker
	$(GO) build -o dist/bin/panel-agent ./cmd/panel-agent
	$(GO) build -o dist/bin/panel-cli ./cmd/panel-cli
	$(GO) build -o dist/bin/panel-updater ./cmd/panel-updater
	$(GO) build -o dist/bin/panel-backup ./cmd/panel-backup
	$(GO) build -o dist/bin/panel-install ./cmd/panel-install
	$(GO) build -o dist/bin/panel-dev ./cmd/panel-dev
	$(GO) build -o dist/bin/panel-smtp-policy ./cmd/panel-smtp-policy
	$(GO) build -o dist/bin/panel-object-store ./cmd/panel-object-store
	cp dist/bin/panel-api dist/bin/kelmor-api
	cp dist/bin/panel-worker dist/bin/kelmor-worker
	cp dist/bin/panel-agent dist/bin/kelmor-agent
	cp dist/bin/panel-cli dist/bin/kelmor-cli
	cp dist/bin/panel-updater dist/bin/kelmor-updater
	cp dist/bin/panel-backup dist/bin/kelmor-backup
	cp dist/bin/panel-install dist/bin/kelmor-install
	cp dist/bin/panel-dev dist/bin/kelmor-dev
	cp dist/bin/panel-smtp-policy dist/bin/kelmor-smtp-policy
	cp dist/bin/panel-object-store dist/bin/kelmor-object-store

package: build
	bash packaging/debian/build.sh

release: lint test build
	@echo "Release artifacts would be signed from CI, not a workstation."

portals:
	cd portals/server && npm install && npm run build
	cd portals/account && npm install && npm run build
	mkdir -p dist/share/portals
	rm -rf dist/share/portals/server dist/share/portals/account
	cp -a portals/server/dist dist/share/portals/server
	cp -a portals/account/dist dist/share/portals/account

dev:
	PANEL_DEV=1 PANEL_API_ADDR=127.0.0.1:$(API_PORT) ./scripts/dev.sh
