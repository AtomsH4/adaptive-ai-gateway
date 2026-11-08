.PHONY: check arch test race vet contracts setup-tools

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
