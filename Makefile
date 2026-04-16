.PHONY: build test lint clean run dev seed proto docker

build:
	go build -o bin/spectra ./cmd/spectra

test:
	go test -v -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

run: build
	./bin/spectra serve

proto:
	protoc --go_out=. --go-grpc_out=. proto/spectra/v1/spectra.proto

docker:
	docker build -t spectra:latest .

dev:
	./start.sh

seed:
	./seed.sh
