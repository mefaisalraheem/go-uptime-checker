.PHONY: build test run fmt vet clean

build:
	go build -o bin/uptime-checker .

test:
	go test ./... -count=1 -race

run:
	go run . -file testdata/urls.txt -concurrency 10 -timeout 5s

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf bin