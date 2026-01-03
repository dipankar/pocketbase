# Introduction

PocketBase is an open source backend that combines a database, authentication system, file storage, and admin UI into a single portable executable. It's designed for developers who need a rapid backend solution without complex infrastructure.

## Core Concepts

### Collections

Collections are similar to database tables. Each collection has a defined schema with fields that determine what data can be stored. PocketBase has three types of collections:

- **Base Collections** - Standard data storage
- **Auth Collections** - User authentication with built-in auth fields
- **View Collections** - Read-only collections based on SQL queries

### Records

Records are the individual data entries within a collection. Every record automatically has:

- `id` - Unique 15-character identifier
- `created` - Creation timestamp
- `updated` - Last modification timestamp

### Fields

Fields define the structure of your data. PocketBase supports many field types:

| Field Type | Description |
|------------|-------------|
| Text | Single or multi-line text |
| Number | Integer or decimal numbers |
| Bool | True/false values |
| Email | Validated email addresses |
| URL | Validated URLs |
| DateTime | Date and time values |
| Select | Single or multiple choice options |
| File | File uploads |
| Relation | Links to other records |
| JSON | Arbitrary JSON data |
| Editor | Rich text (HTML) content |

### API Rules

Each collection has configurable API rules that control access:

- **List/Search** - Who can view multiple records
- **View** - Who can view a single record
- **Create** - Who can create records
- **Update** - Who can modify records
- **Delete** - Who can remove records

Rules use a filter syntax to define permissions based on the authenticated user and record data.

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│              PocketBase                      │
├─────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────────┐ │
│  │ REST API│  │Admin UI │  │ Realtime WS │ │
│  └────┬────┘  └────┬────┘  └──────┬──────┘ │
│       │            │              │         │
│  ┌────┴────────────┴──────────────┴────┐   │
│  │           Core Engine               │   │
│  │  • Collections & Records            │   │
│  │  • Authentication                   │   │
│  │  • File Storage                     │   │
│  │  • Hooks & Events                   │   │
│  └─────────────────┬───────────────────┘   │
│                    │                        │
│  ┌─────────────────┴───────────────────┐   │
│  │         SQLite Database             │   │
│  └─────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

## Use Cases

PocketBase is ideal for:

- **Prototypes & MVPs** - Get a backend running in minutes
- **Mobile Apps** - REST API with authentication and file uploads
- **Small to Medium Projects** - Handle thousands of concurrent users
- **Self-hosted Solutions** - Full control over your data
- **Internal Tools** - Quick admin interfaces for internal data

## Comparison with Alternatives

| Feature | PocketBase | Firebase | Supabase |
|---------|------------|----------|----------|
| Self-hosted | Yes | No | Yes |
| Single executable | Yes | N/A | No |
| Realtime | Yes | Yes | Yes |
| Built-in Auth | Yes | Yes | Yes |
| File Storage | Yes | Yes | Yes |
| Admin UI | Yes | Yes | Yes |
| Open Source | Yes | No | Yes |
| Pricing | Free | Freemium | Freemium |

## Next Steps

- [Install PocketBase](installation.md)
- [Quick Start Guide](quickstart.md)
- [Explore the API](../api/overview.md)
