# Skaffen build targets
# `make` or `make build` — default full build
# `make fast` — skip wasmtime + image (saves ~30s on cached builds)
# `make check` — type-check only (no linking, fastest iteration)
# `make test` — run test suite via nextest
# `make release` — optimized release build (thin LTO)

.PHONY: build fast check test release clean

build:
	cargo build

fast:
	cargo build --no-default-features --features "jemalloc,sqlite-sessions,clipboard"

check:
	cargo check

test:
	cargo nextest run

release:
	cargo build --release

clean:
	cargo clean
