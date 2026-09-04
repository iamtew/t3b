# t3b build recipes for Meat Bags. Default: build the binary.
set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

binary := if os() == "windows" { "t3b.exe" } else { "t3b" }

default: build

# Compile cmd/t3b into ./t3b or ./t3b.exe
build:
    go build -o {{binary}} ./cmd/t3b

# Remove native binary and cross-build output under bin/
[windows]
clean:
    if (Test-Path {{binary}}) { Remove-Item -Force {{binary}} }
    if (Test-Path bin) { Remove-Item -Recurse -Force bin }

[unix]
clean:
    rm -f {{binary}}
    rm -rf bin

# Cross-compile amd64 Windows binary into bin/ (CGO off for portable hosts)
[windows]
build-windows:
    New-Item -ItemType Directory -Force -Path bin | Out-Null
    $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"; go build -o bin/t3b-windows-amd64.exe ./cmd/t3b

[unix]
build-windows:
    mkdir -p bin
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o bin/t3b-windows-amd64.exe ./cmd/t3b

# Cross-compile amd64 Linux binary into bin/ (works from Windows without a C toolchain)
[windows]
build-linux:
    New-Item -ItemType Directory -Force -Path bin | Out-Null
    $env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"; go build -o bin/t3b-linux-amd64 ./cmd/t3b

[unix]
build-linux:
    mkdir -p bin
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/t3b-linux-amd64 ./cmd/t3b

# Both cross builds into bin/
build-all: build-windows build-linux

# Run the bot in the foreground (needs t3b.conf in $PWD or -config)
run *args:
    go run ./cmd/t3b {{args}}

# Refresh module deps and go.sum
tidy:
    go mod tidy

# Run package tests
test:
    go test ./...

# Format all Go sources
fmt:
    go fmt ./...
