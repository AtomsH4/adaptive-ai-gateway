.PHONY: check arch test race vet contracts setup-tools setup-client check-client build

arch:
	go run ./cmd/archcheck

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

setup-tools:
	test -x .venv-tools/bin/python || uv venv --python 3.12 .venv-tools
	uv pip sync --python .venv-tools/bin/python --require-hashes tools/requirements.txt

contracts:
	.venv-tools/bin/python scripts/check_contracts.py

check: arch test race vet contracts

setup-client:
	test -x .venv-client/bin/python || uv venv --python 3.12 .venv-client
	uv pip sync --python .venv-client/bin/python --require-hashes examples/python/requirements.txt

check-client:
	.venv-client/bin/python -m unittest discover -s examples/python -p 'test_*.py'

build:
	go build -o bin/gateway ./cmd/gateway
