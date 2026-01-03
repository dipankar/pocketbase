# PocketBase Documentation

PocketBase is an open source Go backend consisting of:

- Embedded database (SQLite) with realtime subscriptions
- Built-in files and users management
- Convenient Admin dashboard UI
- Simple REST-ish API

**All in a single portable executable!**

## Why PocketBase?

- **Single File Deployment** - Download and run. No dependencies, no configuration files required
- **Realtime Database** - Subscribe to database changes via WebSocket
- **Built-in Auth** - Email/password, OAuth2, and more authentication methods
- **File Storage** - Local or S3-compatible storage for file uploads
- **Admin Dashboard** - Visual interface for managing your data
- **Extensible** - Use as a standalone app or as a Go framework

## Quick Links

<div class="grid cards" markdown>

- :material-download: **[Installation](getting-started/installation.md)**

    Download and install PocketBase on your system

- :material-rocket-launch: **[Quick Start](getting-started/quickstart.md)**

    Get up and running in 5 minutes

- :material-api: **[API Reference](api/overview.md)**

    Explore the REST API endpoints

- :material-puzzle: **[Extending](development/extending-go.md)**

    Build custom functionality with Go or JavaScript

</div>

## System Requirements

- **Operating Systems**: Linux, macOS, Windows, FreeBSD
- **Architecture**: AMD64, ARM64
- **Go Version**: 1.24.0+ (for development/extending)

## Getting Help

- [GitHub Issues](https://github.com/pocketbase/pocketbase/issues) - Report bugs and request features
- [GitHub Discussions](https://github.com/pocketbase/pocketbase/discussions) - Ask questions and share ideas

!!! warning "Pre-release Software"
    PocketBase is still under active development. The API and features may change before the v1.0.0 release. Please check the release notes when upgrading.
