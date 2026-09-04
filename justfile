# t3b build recipes for Meat Bags. Default: build the binary.
set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

binary := if os() == "windows" { "t3b.exe" } else { "t3b" }

default: build

# Compile cmd/t3b into ./t3b or ./t3b.exe
build:
    go build -o {{binary}} ./cmd/t3b

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
