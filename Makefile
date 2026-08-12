.PHONY: build test lint snapshot clean

build:
	go build -trimpath -o semstat .

test:
	go test ./... -race -cover

# gofmt -l reports offending files but always exits 0, so the emptiness of its
# output is the check. Without this `make lint` passes where CI fails.
lint:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt needed on: $$unformatted"; exit 1; fi
	go vet ./...

# Exercise the full release build locally without publishing anything.
snapshot:
	goreleaser release --snapshot --clean --skip=sign,sbom

clean:
	rm -rf dist semstat
