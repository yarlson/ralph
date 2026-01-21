# PRD Writing Guide for Greenfield Projects

This guide explains how to write effective PRDs for new projects built from scratch. Unlike feature PRDs for existing projects, greenfield PRDs must define architecture, tech stack, and project structure upfront since there's no existing codebase to reference.

## Key Principles

### 1. Define the Tech Stack Explicitly

Ralph cannot make technology choices. Specify languages, frameworks, libraries, and tooling upfront.

**Bad:**

> Build a performant solution with good error handling.

**Good:**

> Build with Go 1.22, using `cobra` for CLI, `viper` for configuration, and `zap` for structured logging. Use `testify` for assertions. Target Linux and macOS.

### 2. Specify Project Structure

Without an existing codebase, you must define where files should live.

**Bad:**

> Create a modular architecture.

**Good:**

> Organize code as:
>
> - `cmd/` - Entry points with thin wrappers
> - `internal/` - Private application code (business logic)
> - `pkg/` - Public libraries for external consumption
> - `config/` - Configuration loading and validation

### 3. Define Interfaces and Contracts

New projects need clear boundaries between components. Specify how parts communicate.

**Bad:**

> The processor should handle data from the collector.

**Good:**

> Processor interface:
>
> - Accepts `Event` struct: `{ID string, Timestamp time.Time, Payload []byte}`
> - Returns `Result` or `error`
> - Called by Collector via direct method invocation (no queue for v1)
> - Must complete within 5 seconds or return timeout error

### 4. Make All Decisions Upfront

Ralph's task executor runs autonomously and cannot ask clarifying questions. Every design decision must be resolved in the PRD.

**Bad:**

> We might want to support multiple storage backends.

**Good:**

> Storage: PostgreSQL only for v1. Interface defined to allow future backends, but only PostgreSQL implementation built. No abstraction layer beyond repository pattern.

---

## PRD Structure for Greenfield Projects

### Required Sections

```markdown
# [Project Name] - PRD

## Overview

[What is this project? What problem does it solve? Who uses it?]

## Goals

[Measurable outcomes: what success looks like]

## Non-Goals

[What's explicitly out of scope for v1]

## Tech Stack

[CRITICAL: All technology choices must be specified]

## Project Structure

[CRITICAL: Define file/folder organization]

## Architecture

[High-level component design and interactions]

## Data Model

[Core entities, schemas, relationships]

## Interfaces & Contracts

[APIs, protocols, data formats between components]

## Error Handling

[How errors propagate, logging, recovery]

## Configuration

[What's configurable, defaults, environment variables]

## Verification Commands

[How to verify the implementation]
```

### Recommended Sections

```markdown
## Security Considerations

[Authentication, authorization, secrets management]

## Performance Requirements

[Latency, throughput, resource constraints]

## Testing Strategy

[Unit, integration, end-to-end approaches]

## Deployment & Operations

[Build, release, monitoring, observability]

## Rollout Plan

[Phased delivery if applicable]

## Risks & Mitigations

[Known risks and how to handle them]

## Decisions Made

[Key decisions with rationale]
```

---

## The Critical Section: Tech Stack

This section replaces "Existing Code Context" for greenfield projects. It defines the foundation.

### What to Include

```markdown
## Tech Stack

### Language & Runtime

- **Go 1.22** (or specify version)
- Target platforms: Linux amd64, Darwin arm64
- Minimum supported version documented in README

### Core Dependencies

- **cobra** v1.8 - CLI framework
- **viper** v1.18 - Configuration management
- **zap** v1.27 - Structured logging
- **pgx** v5 - PostgreSQL driver (if applicable)

### Testing

- **testify** v1.9 - Assertions and mocking
- Standard library `testing` package
- Table-driven tests preferred

### Development Tools

- **golangci-lint** v1.56 - Linting
- **gofmt** - Formatting (standard)
- **go mod** - Dependency management

### Build & Release

- **goreleaser** - Cross-platform builds (if distributing binaries)
- Or: `go build` with manual GOOS/GOARCH

### Optional: External Services

- **PostgreSQL 15** - Primary data store
- **Redis 7** - Caching (if needed)
- Document connection requirements
```

### Why This Matters

Without explicit tech stack definition, Ralph might:

- Choose incompatible library versions
- Mix different patterns (e.g., multiple logging libraries)
- Create inconsistent project structure
- Use deprecated or unsuitable tools

---

## Project Structure Section

Define the complete folder structure before any implementation.

```markdown
## Project Structure
```

├── cmd/
│ └── myapp/ # Main entry point
│ └── main.go
│
├── internal/ # Private application code
│ ├── config/ # Configuration loading
│ │ └── config.go
│ ├── service/ # Business logic
│ │ ├── service.go
│ │ └── service_test.go
│ ├── repository/ # Data access layer
│ │ ├── repository.go
│ │ └── postgres.go
│ └── model/ # Domain types
│ └── model.go
│
├── pkg/ # Public libraries (if any)
│ └── client/ # Client library for consumers
│
├── migrations/ # Database migrations (if applicable)
│ ├── 001_initial.up.sql
│ └── 001_initial.down.sql
│
├── scripts/ # Build/deploy scripts
│ └── build.sh
│
├── .golangci.yml # Linter configuration
├── go.mod
├── go.sum
├── Makefile # Common tasks
└── README.md

```

### Structure Rationale

Document why this structure was chosen:

- `internal/` prevents external imports, enforcing encapsulation
- `cmd/` allows multiple entry points if needed later
- Flat package structure within `internal/` for simplicity
- No `pkg/` if nothing is intended for external consumption

```

---

## Architecture Section

Describe how components interact at a high level.

```markdown
## Architecture

### Component Overview
```

┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ CLI │────▶│ Service │────▶│ Repository │
│ (cmd/) │ │ (internal/) │ │ (internal/) │
└─────────────┘ └─────────────┘ └─────────────┘
│
▼
┌─────────────┐
│ PostgreSQL │
└─────────────┘

```

### Component Responsibilities

#### CLI (`cmd/myapp/`)
- Parse command-line arguments
- Initialize dependencies (config, logger, DB connection)
- Call service layer
- Handle exit codes and output formatting

#### Service (`internal/service/`)
- Core business logic
- Orchestrates repository calls
- Validates input
- Returns domain errors (not infrastructure errors)

#### Repository (`internal/repository/`)
- Data access abstraction
- PostgreSQL implementation
- Handles connection pooling
- Translates SQL errors to domain errors

### Dependency Flow

- Dependencies flow inward: CLI → Service → Repository
- Interfaces defined at consumer (Service defines Repository interface)
- No circular dependencies
```

---

## Data Model Section

Define all core entities with their fields, types, and relationships.

````markdown
## Data Model

### Core Entities

#### User

| Field     | Type         | Constraints      | Description       |
| --------- | ------------ | ---------------- | ----------------- |
| ID        | UUID         | PRIMARY KEY      | Unique identifier |
| Email     | VARCHAR(255) | UNIQUE, NOT NULL | User email        |
| Name      | VARCHAR(100) | NOT NULL         | Display name      |
| Status    | VARCHAR(20)  | NOT NULL         | active, suspended |
| CreatedAt | TIMESTAMP    | NOT NULL         | Creation time     |
| UpdatedAt | TIMESTAMP    | NOT NULL         | Last modification |

#### Go Struct

```go
type User struct {
    ID        uuid.UUID
    Email     string
    Name      string
    Status    UserStatus
    CreatedAt time.Time
    UpdatedAt time.Time
}

type UserStatus string

const (
    UserStatusActive    UserStatus = "active"
    UserStatusSuspended UserStatus = "suspended"
)
```
````

### Relationships

- One User has many Items (1:N)
- Items belong to exactly one User

### Migrations

- Located in `migrations/`
- Use golang-migrate or similar
- Up and down migrations required
- Naming: `NNN_description.up.sql`, `NNN_description.down.sql`

`````

---

## Interfaces & Contracts Section

Define how components communicate with explicit types.

````markdown
## Interfaces & Contracts

### Service Interface

```go
// UserService defines the user business logic contract.
type UserService interface {
    Create(ctx context.Context, req CreateUserRequest) (*User, error)
    Get(ctx context.Context, id uuid.UUID) (*User, error)
    List(ctx context.Context, opts ListOptions) ([]*User, error)
    Update(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*User, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
`````

### Repository Interface

```go
// UserRepository defines the data access contract.
type UserRepository interface {
    Insert(ctx context.Context, user *User) error
    FindByID(ctx context.Context, id uuid.UUID) (*User, error)
    FindAll(ctx context.Context, opts QueryOptions) ([]*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### Request/Response Types

```go
type CreateUserRequest struct {
    Email string `json:"email" validate:"required,email"`
    Name  string `json:"name" validate:"required,max=100"`
}

type ListOptions struct {
    Offset int
    Limit  int    // Default: 20, Max: 100
    Status string // Filter by status (optional)
}
```

### Error Types

```go
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrInvalidInput  = errors.New("invalid input")
)
```

````

---

## Error Handling Section

Define error propagation, logging, and recovery strategies.

```markdown
## Error Handling

### Error Wrapping

All errors wrapped with context using `fmt.Errorf`:

```go
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}
````

### Domain vs Infrastructure Errors

- Domain errors: `ErrNotFound`, `ErrInvalidInput` (defined in `internal/model/errors.go`)
- Infrastructure errors: wrapped and logged, not exposed to callers
- Service layer translates infrastructure errors to domain errors

### Logging Strategy

- Use structured logging (zap)
- Log at error boundaries (service layer)
- Include request ID, user ID, operation name
- Don't log sensitive data (passwords, tokens)

```go
logger.Error("failed to create user",
    zap.String("email", req.Email),
    zap.Error(err),
)
```

### Panic Recovery

- No panics in application code
- Recover only at top-level (HTTP middleware, CLI main)
- Log recovered panics with stack trace

````

---

## Configuration Section

Define what's configurable and how configuration is loaded.

```markdown
## Configuration

### Configuration Sources (Priority Order)

1. Command-line flags (highest priority)
2. Environment variables
3. Configuration file (`config.yaml`)
4. Defaults (lowest priority)

### Configuration Structure

```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Log      LogConfig
}

type ServerConfig struct {
    Port         int           `default:"8080"`
    ReadTimeout  time.Duration `default:"30s"`
    WriteTimeout time.Duration `default:"30s"`
}

type DatabaseConfig struct {
    Host     string `required:"true"`
    Port     int    `default:"5432"`
    User     string `required:"true"`
    Password string `required:"true"` // From env: DB_PASSWORD
    Database string `required:"true"`
    SSLMode  string `default:"disable"`
}

type LogConfig struct {
    Level  string `default:"info"` // debug, info, warn, error
    Format string `default:"json"` // json, console
}
````

### Environment Variables

| Variable      | Description        | Required | Default |
| ------------- | ------------------ | -------- | ------- |
| `DB_HOST`     | Database hostname  | Yes      | -       |
| `DB_PORT`     | Database port      | No       | 5432    |
| `DB_USER`     | Database username  | Yes      | -       |
| `DB_PASSWORD` | Database password  | Yes      | -       |
| `DB_NAME`     | Database name      | Yes      | -       |
| `LOG_LEVEL`   | Logging level      | No       | info    |
| `SERVER_PORT` | Server listen port | No       | 8080    |

### Configuration File

```yaml
# config.yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  host: localhost
  port: 5432
  database: myapp
  ssl_mode: disable

log:
  level: info
  format: json
```

````

---

## Verification Commands Section

Define how to verify the implementation works.

```markdown
## Verification Commands

These commands must pass after implementation:

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# Lint
golangci-lint run

# Format check
gofmt -l . | grep -q . && echo "Files need formatting" && exit 1 || echo "OK"
````

### Test Requirements

- All packages have `*_test.go` files
- Coverage target: > 70% for business logic (`internal/service/`)
- Table-driven tests preferred
- No external dependencies in unit tests (mock interfaces)

### Integration Test Requirements

```bash
# Run integration tests (requires database)
go test -tags=integration ./...
```

- Integration tests use build tag `// +build integration`
- Require running database (document setup)
- Clean up test data after runs

````

---

## Common Mistakes in Greenfield PRDs

### 1. No Tech Stack Definition

**Bad:**

> Build a fast and reliable service.

**Good:**

> Build with Go 1.22, using standard library HTTP server, `pgx` for PostgreSQL, and `zap` for logging. Use `testify` for test assertions.

### 2. Undefined Component Boundaries

**Bad:**

> Create a modular system with clean separation of concerns.

**Good:**

> Three-layer architecture:
>
> - Handler layer (`internal/handler/`): HTTP request parsing, response formatting
> - Service layer (`internal/service/`): Business logic, validation
> - Repository layer (`internal/repository/`): Data access, SQL queries
>
> Dependencies flow inward. Service defines repository interface.

### 3. Missing Error Handling Strategy

**Bad:**

> Handle errors appropriately.

**Good:**

> Error handling:
>
> - Wrap errors with context: `fmt.Errorf("create user: %w", err)`
> - Define domain errors in `internal/model/errors.go`
> - Service layer translates infrastructure errors
> - Log at error boundaries, not at every layer

### 4. Ambiguous Data Model

**Bad:**

> Store user information in the database.

**Good:**

> Users table:
>
> - `id` UUID PRIMARY KEY
> - `email` VARCHAR(255) UNIQUE NOT NULL
> - `name` VARCHAR(100) NOT NULL
> - `status` VARCHAR(20) NOT NULL DEFAULT 'active'
> - `created_at` TIMESTAMP NOT NULL DEFAULT NOW()
> - `updated_at` TIMESTAMP NOT NULL DEFAULT NOW()
>
> Index: `idx_users_email` on `email`

### 5. No Project Structure

**Bad:**

> Organize code logically.

**Good:**

> Folder structure:
>
> - `cmd/myapp/main.go` - Entry point
> - `internal/config/` - Configuration loading
> - `internal/handler/` - Request handlers
> - `internal/service/` - Business logic
> - `internal/repository/` - Data access
> - `internal/model/` - Domain types

### 6. Unresolved Decisions

**Bad:**

> We might want to add caching later.

**Good:**

> Caching: Not implemented in v1. Repository interface is cache-friendly (can wrap later). No caching abstraction built upfront.

---

## Template: Greenfield Project PRD

```markdown
# [Project Name] - PRD

## Overview

[2-3 sentences: What is this project? What problem does it solve?]

## Goals

- [Measurable goal 1]
- [Measurable goal 2]
- [Measurable goal 3]

## Non-Goals

- [Explicitly excluded 1]
- [Explicitly excluded 2]
- [Explicitly excluded 3]

## Tech Stack

### Language & Runtime

- [Language and version]
- Target platforms: [list]

### Core Dependencies

- [Dependency 1] - [purpose]
- [Dependency 2] - [purpose]
- [Dependency 3] - [purpose]

### Development Tools

- [Linter]
- [Formatter]
- [Test framework]

### External Services (if any)

- [Database/cache/queue with version]

## Project Structure

````

[folder tree]

````

## Architecture

### Component Overview

[Diagram or description of component relationships]

### Component Responsibilities

#### [Component 1]

- [Responsibility 1]
- [Responsibility 2]

#### [Component 2]

- [Responsibility 1]
- [Responsibility 2]

## Data Model

### [Entity 1]

| Field | Type | Constraints | Description |
| ----- | ---- | ----------- | ----------- |
| ...   | ...  | ...         | ...         |

### Relationships

- [Describe relationships between entities]

## Interfaces & Contracts

### [Interface 1]

```go
type [InterfaceName] interface {
    // Methods with signatures
}
````

### Request/Response Types

```go
type [TypeName] struct {
    // Fields
}
```

## Error Handling

### Error Types

- [Domain error 1]: [when used]
- [Domain error 2]: [when used]

### Logging

- [Logging strategy]

## Configuration

### Environment Variables

| Variable | Description | Required | Default |
| -------- | ----------- | -------- | ------- |
| ...      | ...         | ...      | ...     |

### Configuration File

```yaml
# Example configuration
```

## Verification Commands

```bash
go build ./...
go test ./...
golangci-lint run
```

### Test Requirements

- [Coverage targets]
- [Test organization]

## Performance Requirements

- [Latency targets]
- [Throughput requirements]
- [Resource constraints]

## Security Considerations

- [Authentication approach]
- [Secrets management]
- [Input validation]

## Rollout Plan

1. **Phase 1**: [Core functionality]
2. **Phase 2**: [Extended functionality]

## Risks & Mitigations

### Risk: [Description]

**Mitigation**: [How to handle]

## Decisions Made

- [Decision 1]: Chose X over Y because [reason]
- [Decision 2]: Chose A over B because [reason]

````

---

## Checklist Before Submitting Greenfield PRD to Ralph

- [ ] Tech stack fully specified (language version, all dependencies)
- [ ] Project structure defined (complete folder tree)
- [ ] Architecture documented (components and interactions)
- [ ] Data model complete (all entities, fields, relationships)
- [ ] Interfaces defined (Go interfaces or protocol specs)
- [ ] Error handling strategy specified
- [ ] Configuration documented (env vars, defaults)
- [ ] Verification commands specified
- [ ] Non-goals clearly exclude future phases
- [ ] No unresolved decisions ("we might want to", "TBD")
- [ ] Performance requirements have specific numbers
- [ ] Security considerations addressed

---

## Minimal Greenfield PRD Example

For smaller projects, you can use a condensed format:

```markdown
# URL Shortener - PRD

## Overview

A URL shortening service that creates short links and tracks click analytics.

## Goals

- Shorten URLs with custom or generated slugs
- Redirect short URLs to original destinations
- Track click counts per link

## Non-Goals

- User accounts (anonymous usage only)
- Custom domains
- Link expiration

## Tech Stack

- Go 1.22
- Standard library HTTP server
- SQLite for storage (single binary deployment)
- `testify` for testing

## Project Structure

````

cmd/shortener/main.go
internal/
├── handler/ # HTTP handlers
├── service/ # Business logic
├── store/ # SQLite storage
└── model/ # Types

````

## Data Model

### Link

| Field       | Type    | Description       |
| ----------- | ------- | ----------------- |
| slug        | TEXT PK | Short URL slug    |
| url         | TEXT    | Original URL      |
| clicks      | INTEGER | Click count       |
| created_at  | TEXT    | ISO8601 timestamp |

## Endpoints

- `POST /shorten` - Create short link
- `GET /:slug` - Redirect to original URL
- `GET /:slug/stats` - Get click count

## Verification

```bash
go build ./...
go test ./...
````

```

---

This guide ensures your greenfield PRDs provide Ralph with everything needed to build a complete, well-structured project from scratch.
```
