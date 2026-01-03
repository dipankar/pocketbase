# Installation

PocketBase is distributed as a single executable file. Choose your preferred installation method below.

## Download Binary

Download the latest release for your platform from the [GitHub Releases](https://github.com/pocketbase/pocketbase/releases) page.

### Linux

```bash
# Download (replace with latest version)
wget https://github.com/pocketbase/pocketbase/releases/download/v0.x.x/pocketbase_0.x.x_linux_amd64.zip

# Extract
unzip pocketbase_0.x.x_linux_amd64.zip

# Make executable
chmod +x pocketbase

# Run
./pocketbase serve
```

### macOS

```bash
# Download (replace with latest version)
curl -L -o pocketbase.zip https://github.com/pocketbase/pocketbase/releases/download/v0.x.x/pocketbase_0.x.x_darwin_amd64.zip

# Extract
unzip pocketbase.zip

# Run
./pocketbase serve
```

For Apple Silicon (M1/M2), use `darwin_arm64` instead.

### Windows

1. Download the Windows release (`pocketbase_x.x.x_windows_amd64.zip`)
2. Extract the zip file
3. Open Command Prompt or PowerShell in the extracted folder
4. Run: `.\pocketbase.exe serve`

## Using Docker

```dockerfile
FROM alpine:latest

ARG PB_VERSION=0.x.x

RUN apk add --no-cache \
    unzip \
    ca-certificates

# Download and unzip PocketBase
ADD https://github.com/pocketbase/pocketbase/releases/download/v${PB_VERSION}/pocketbase_${PB_VERSION}_linux_amd64.zip /tmp/pb.zip
RUN unzip /tmp/pb.zip -d /pb/

EXPOSE 8080

# Start PocketBase
CMD ["/pb/pocketbase", "serve", "--http=0.0.0.0:8080"]
```

Build and run:

```bash
docker build -t pocketbase .
docker run -p 8080:8080 -v /path/to/data:/pb/pb_data pocketbase
```

## Building from Source

Requirements:

- Go 1.24.0 or later
- Git

```bash
# Clone the repository
git clone https://github.com/pocketbase/pocketbase.git
cd pocketbase

# Build
go build -o pocketbase ./examples/base

# Run
./pocketbase serve
```

## As a Go Dependency

To use PocketBase as a framework in your Go project:

```bash
go get github.com/pocketbase/pocketbase
```

Example `main.go`:

```go
package main

import (
    "log"

    "github.com/pocketbase/pocketbase"
)

func main() {
    app := pocketbase.New()

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Verifying Installation

After installation, run:

```bash
./pocketbase serve
```

You should see output similar to:

```
Server started at http://127.0.0.1:8090
 - REST API: http://127.0.0.1:8090/api/
 - Admin UI: http://127.0.0.1:8090/_/
```

Open `http://127.0.0.1:8090/_/` in your browser to access the Admin UI.

## Directory Structure

When PocketBase runs, it creates a `pb_data` directory:

```
pb_data/
├── data.db         # Main SQLite database
├── aux.db          # Auxiliary database
├── storage/        # Uploaded files
└── backups/        # Database backups
```

!!! tip "Data Persistence"
    Always mount or preserve the `pb_data` directory when using Docker or deploying to production.

## Next Steps

- [Quick Start Guide](quickstart.md)
- [Configuration Options](configuration.md)
- [CLI Commands Reference](cli-commands.md)
