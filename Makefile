.PHONY: build clean

build:
	go build -ldflags="-w -s" -o bin/tgctl ./cmd/tgctl/

clean:
	rm -rf bin/

cross:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/tgctl-darwin-arm64 ./cmd/tgctl/
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/tgctl-darwin-amd64 ./cmd/tgctl/
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/tgctl-linux-amd64 ./cmd/tgctl/
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/tgctl-linux-arm64 ./cmd/tgctl/
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/tgctl-windows-amd64.exe ./cmd/tgctl/
