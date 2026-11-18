.PHONY: build build-windows run clean

build:
	go build -o just-proxy .

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o just-proxy.exe .

run:
	go run . -config config.yaml

clean:
	rm -f just-proxy just-proxy.exe
