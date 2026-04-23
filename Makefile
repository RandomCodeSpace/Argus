.PHONY: build test vet check setup-hooks ui-install ui-build dev-ui loadtest loadtest-build

ui-install:
	cd ui && npm install

ui-build:
	cd ui && npm run build

build: ui-build
	CGO_ENABLED=0 go build ./...

vet:
	go vet ./...

test:
	CGO_ENABLED=1 go test -race -timeout 120s ./...

## check runs the same steps as CI: build → vet → test
check: build vet test

dev-ui:
	cd ui && npm run dev

loadtest-build:
	CGO_ENABLED=0 go build -tags loadtest -o bin/loadsim ./test/loadsim

loadtest: loadtest-build
	@echo "Running 200-service load simulator (60s) against localhost:4317..."
	./bin/loadsim

## setup-hooks installs the pre-commit hook into .git/hooks
setup-hooks:
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	@echo "✅ pre-commit hook installed"
