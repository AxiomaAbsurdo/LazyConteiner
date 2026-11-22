.PHONY: setup run demo test build check benchmark test-live test-input package package-check

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
	cd fleet && bun test
	python3 scripts/dev.py --test

check:
	./scripts/exact-run.sh check

benchmark:
	python3 scripts/benchmark.py

test-live:
	python3 scripts/live_check.py

test-input: build
	python3 scripts/native-input-test.py

package:
	python3 scripts/build-deb.py

package-check:
	python3 scripts/check-deb.py dist/lazyconteiner_1.0.0-1_amd64.deb
