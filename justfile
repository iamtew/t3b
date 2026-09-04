# t3b build recipes for Meat Bags. Default: build the binary.
set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

binary := if os() == "windows" { "t3b.exe" } else { "t3b" }
# Linker -X path for the stamped Version string (Justfile fills it at build time).
version_pkg := "github.com/iamtew/t3b/internal/version.Version"

default: build

# Compile cmd/t3b into ./t3b or ./t3b.exe (stamps Version from git).
[windows]
build:
    $short = (git rev-parse --short HEAD).Trim(); $tag = $null; git describe --tags --exact-match HEAD 2>$null | ForEach-Object { if ($_) { $tag = $_.Trim() } }; if ($tag) { $ver = "$tag-$short" } else { $ver = $short }; if (git status --porcelain) { $ver = "$ver-dirty" }; go build -ldflags "-X {{version_pkg}}=$ver" -o {{binary}} ./cmd/t3b

[unix]
build:
    short=$(git rev-parse --short HEAD); if tag=$(git describe --tags --exact-match HEAD 2>/dev/null); then ver="${tag}-${short}"; else ver="${short}"; fi; if [ -n "$(git status --porcelain)" ]; then ver="${ver}-dirty"; fi; go build -ldflags "-X {{version_pkg}}=${ver}" -o {{binary}} ./cmd/t3b

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
    $short = (git rev-parse --short HEAD).Trim(); $tag = $null; git describe --tags --exact-match HEAD 2>$null | ForEach-Object { if ($_) { $tag = $_.Trim() } }; if ($tag) { $ver = "$tag-$short" } else { $ver = $short }; if (git status --porcelain) { $ver = "$ver-dirty" }; $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"; go build -ldflags "-X {{version_pkg}}=$ver" -o bin/t3b-windows-amd64.exe ./cmd/t3b

[unix]
build-windows:
    mkdir -p bin
    short=$(git rev-parse --short HEAD); if tag=$(git describe --tags --exact-match HEAD 2>/dev/null); then ver="${tag}-${short}"; else ver="${short}"; fi; if [ -n "$(git status --porcelain)" ]; then ver="${ver}-dirty"; fi; GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X {{version_pkg}}=${ver}" -o bin/t3b-windows-amd64.exe ./cmd/t3b

# Cross-compile amd64 Linux binary into bin/ (works from Windows without a C toolchain)
[windows]
build-linux:
    New-Item -ItemType Directory -Force -Path bin | Out-Null
    $short = (git rev-parse --short HEAD).Trim(); $tag = $null; git describe --tags --exact-match HEAD 2>$null | ForEach-Object { if ($_) { $tag = $_.Trim() } }; if ($tag) { $ver = "$tag-$short" } else { $ver = $short }; if (git status --porcelain) { $ver = "$ver-dirty" }; $env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"; go build -ldflags "-X {{version_pkg}}=$ver" -o bin/t3b-linux-amd64 ./cmd/t3b

[unix]
build-linux:
    mkdir -p bin
    short=$(git rev-parse --short HEAD); if tag=$(git describe --tags --exact-match HEAD 2>/dev/null); then ver="${tag}-${short}"; else ver="${short}"; fi; if [ -n "$(git status --porcelain)" ]; then ver="${ver}-dirty"; fi; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X {{version_pkg}}=${ver}" -o bin/t3b-linux-amd64 ./cmd/t3b

# Both cross builds into bin/
build-all: build-windows build-linux

# Run the bot in the foreground (needs *t3b.conf in $PWD, or -config / -config_write)
# go run uses the version package VCS fallback (no Justfile stamp).
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
