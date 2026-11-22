.PHONY: setup run demo test build check

setup:
	./scripts/exact-setup.sh

run:
	python3 scripts/dev.py

demo:
	python3 scripts/dev.py --demo

build:
	mkdir -p .runtime
	cd backend && go build -o ../.runtime/backend .
	./scripts/exact-run.sh build

test:
	cd backend && go test ./...
	cd fleet && bun test app.test.ts
	python3 scripts/dev.py --test

check:
	./scripts/exact-run.sh check
