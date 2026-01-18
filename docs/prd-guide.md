# PRD Writing Guide for Ralph

This guide explains how to write effective PRDs (Product Requirements Documents) for new features in existing projects. Ralph decomposes PRDs into executable task hierarchies, so structure and specificity matter.

## Key Principles

### 1. Be Explicit, Not Ambiguous

Ralph's task executor (Claude Code) runs autonomously—it cannot ask clarifying questions. Every decision must be made upfront in the PRD.

**Bad:**

> Add caching to improve performance.

**Good:**

> Add Redis-based caching for the `/api/users/:id` endpoint with a 5-minute TTL. Cache invalidation occurs on user update.

### 2. Reference Existing Code

Unlike greenfield projects, features in existing codebases must integrate with current architecture. Always reference:

- Existing files to modify
- Patterns already in use
- Conventions to follow

**Bad:**

> Create a new API endpoint for user preferences.

**Good:**

> Create `GET/PUT /api/users/:id/preferences` endpoints following the pattern in `internal/api/users.go`. Use the existing `UserRepository` for data access.

### 3. Specify File Paths

Ralph creates tasks that target specific files. The more explicit you are, the better the task decomposition.

**Bad:**

> Add validation to user inputs.

**Good:**

> Add validation to `internal/api/handlers/user.go`:
>
> - Email format validation using existing `pkg/validation/email.go`
> - Password strength check (min 8 chars, 1 uppercase, 1 number)
> - Return 400 Bad Request with field-specific errors

### 4. Keep Scope Tight

Each PRD should represent a cohesive feature. Avoid bundling unrelated changes.

**Bad:** PRD that includes "user auth + payment system + email notifications + admin dashboard"

**Good:** One PRD per feature area. Create dependencies between PRDs if needed.

---

## PRD Structure

### Required Sections

```markdown
# Feature Name - PRD

## Overview

[1-2 paragraphs: What is this feature? Why are we building it?]

## Goals

[Bullet list: What success looks like]

## Non-Goals

[Bullet list: What's explicitly OUT of scope—prevents scope creep]

## Requirements

### Functional Requirements

[Numbered sections with specific behaviors]

### Non-Functional Requirements

[Performance, security, reliability constraints]

## Existing Code Context

[CRITICAL for existing projects—see below]

## Data Model

[Schema changes, new tables/fields]

## API Endpoints (if applicable)

[Request/response formats]

## Verification Commands

[How to verify the implementation works]
```

### Recommended Sections

```markdown
## User Journeys

[Step-by-step flows for key scenarios]

## Tech Stack / Dependencies

[Libraries, services, tools to use]

## Rollout Plan

[Phased delivery if applicable]

## Risks & Mitigations

[Known risks and how to handle them]

## Open Questions

[Decisions you've made with rationale—NOT actual open questions]
```

---

## The Critical Section: Existing Code Context

This section is what differentiates a feature PRD from a greenfield PRD. It gives Ralph the context needed to integrate properly.

### What to Include

```markdown
## Existing Code Context

### Project Structure

- API handlers: `internal/api/handlers/`
- Business logic: `internal/services/`
- Data access: `internal/repository/`
- Models: `internal/models/`
- Config: `internal/config/config.go`

### Relevant Files

These files will be modified or extended:

- `internal/api/router.go` - Add new routes
- `internal/services/user_service.go` - Add preference methods
- `internal/repository/user_repo.go` - Add preference queries

### Patterns to Follow

- Use `github.com/go-chi/chi` for routing (see existing handlers)
- Use `sqlx` for database queries (see `internal/repository/`)
- Error handling: wrap with `fmt.Errorf("context: %w", err)`
- Validation: use `github.com/go-playground/validator`

### Database

- Using PostgreSQL
- Migrations in `migrations/` using golang-migrate
- Connection pool in `internal/db/db.go`

### Testing Patterns

- Unit tests: `*_test.go` alongside source files
- Integration tests: `internal/integration/`
- Use `testify/assert` and `testify/require`
- Mock interfaces using `internal/mocks/` (generated with mockery)

### Configuration

- Config loaded from environment via `internal/config/`
- Add new config fields to `Config` struct
- Document in `.env.example`
```

### Why This Matters

Without this context, Ralph might:

- Create files in wrong locations
- Use different patterns than existing code
- Miss integration points
- Break existing functionality

---

## Writing Effective Requirements

### Functional Requirements

Structure by feature area with numbered sections (helps with traceability):

```markdown
### Functional Requirements

#### 8.1 User Preferences Storage

- Users can store key-value preferences (theme, language, notifications)
- Preferences are persisted in PostgreSQL `user_preferences` table
- Default preferences applied on user creation
- Modify `internal/repository/user_repo.go` to add preference methods

#### 8.2 Preferences API

- GET `/api/users/:id/preferences` - Returns all preferences
- PUT `/api/users/:id/preferences` - Updates preferences (partial update)
- Add handlers to `internal/api/handlers/preferences.go` (new file)
- Register routes in `internal/api/router.go`
- Use existing auth middleware from `internal/api/middleware/auth.go`

#### 8.3 Preferences Validation

- Theme: must be "light" or "dark"
- Language: ISO 639-1 code, validated against supported languages
- Notifications: boolean
- Return 400 with validation errors for invalid values
```

### Non-Functional Requirements

Be specific with numbers:

```markdown
### Non-Functional Requirements

#### Performance

- GET preferences: < 50ms p95 (simple key lookup)
- PUT preferences: < 100ms p95
- Add database index on `user_preferences.user_id`

#### Security

- Users can only access their own preferences (enforce in middleware)
- Sanitize preference values to prevent XSS
- Log preference changes for audit trail

#### Reliability

- Graceful handling if preferences table is unavailable
- Return sensible defaults if user has no preferences set
```

---

## Data Model Section

Always show the full schema, not just "add a preferences table":

```markdown
## Data Model

### user_preferences Table (New)

| Column              | Type        | Constraints                    | Description       |
| ------------------- | ----------- | ------------------------------ | ----------------- |
| id                  | UUID        | PRIMARY KEY                    | Unique identifier |
| user_id             | UUID        | FOREIGN KEY (users.id), UNIQUE | Owner             |
| theme               | VARCHAR(10) | DEFAULT 'light'                | UI theme          |
| language            | VARCHAR(5)  | DEFAULT 'en'                   | ISO 639-1 code    |
| email_notifications | BOOLEAN     | DEFAULT true                   | Email opt-in      |
| created_at          | TIMESTAMP   | NOT NULL                       | Creation time     |
| updated_at          | TIMESTAMP   | NOT NULL                       | Last update       |

### Migration

Create migration file: `migrations/000X_add_user_preferences.up.sql`

### Changes to Existing Tables

None required—preferences linked via foreign key.
```

---

## API Endpoints Section

Include full request/response examples:

```markdown
## API Endpoints

### GET /api/users/:id/preferences

**Request:**
```

Headers:
Authorization: Bearer <jwt-token>

````

**Response (200 OK):**
```json
{
  "theme": "dark",
  "language": "en",
  "emailNotifications": true
}
````

**Response (404 Not Found):**

```json
{
  "error": "User not found"
}
```

### PUT /api/users/:id/preferences

**Request:**

```json
{
  "theme": "dark",
  "emailNotifications": false
}
```

Note: Partial updates supported—omitted fields unchanged.

**Response (200 OK):**

```json
{
  "theme": "dark",
  "language": "en",
  "emailNotifications": false
}
```

**Response (400 Bad Request):**

```json
{
  "error": "Validation failed",
  "fields": {
    "theme": "must be 'light' or 'dark'"
  }
}
```

````

---

## Verification Commands

Tell Ralph how to verify the implementation:

```markdown
## Verification Commands

These commands must pass after implementation:

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/api/handlers/...
go test ./internal/repository/...

# Build check
go build ./...

# Lint
golangci-lint run

# Integration tests (if applicable)
go test -tags=integration ./internal/integration/...
````

### Manual Verification

After implementation, verify:

1. Create user via existing registration flow
2. GET /api/users/:id/preferences returns defaults
3. PUT /api/users/:id/preferences updates values
4. Preferences persist across sessions

````

---

## Common Mistakes

### 1. Vague Scope

**Bad:**
> Improve the user experience.

**Good:**
> Add loading spinners to the `/users` list page when fetching data. Show error toast on API failure. Use existing `Spinner` component from `src/components/ui/`.

### 2. Missing Integration Points

**Bad:**
> Add a notification service.

**Good:**
> Add notification service in `internal/services/notification.go`:
> - Inject into `UserService` via constructor
> - Call on user registration (modify `RegisterUser` method)
> - Call on password reset (modify `ResetPassword` method)
> - Use existing `EmailClient` from `internal/email/client.go`

### 3. Undefined Decisions

**Bad:**
> We might want to support multiple notification channels.

**Good:**
> Notification channels for v1: email only. Architecture supports adding SMS/push later via `Notifier` interface, but only `EmailNotifier` implemented now.

### 4. No File Specificity

**Bad:**
> Update the API to support filtering.

**Good:**
> Add query parameter filtering to `GET /api/users`:
> - Modify `internal/api/handlers/user.go:ListUsers`
> - Add filter parsing in `internal/api/filters/user_filter.go` (new file)
> - Supported filters: `status`, `created_after`, `created_before`
> - Update `internal/repository/user_repo.go:List` to accept filters

### 5. Bundling Unrelated Work

**Bad:** One PRD covering authentication, payments, and reporting.

**Good:** Three separate PRDs:
- `auth-feature.md` - Authentication system
- `payments-feature.md` - Payment processing (depends on auth)
- `reporting-feature.md` - Usage reporting (depends on auth)

---

## Template: Feature PRD for Existing Project

```markdown
# [Feature Name] - PRD

## Overview

[2-3 sentences: What is this feature and why does it matter?]

## Goals

- [Measurable goal 1]
- [Measurable goal 2]
- [Measurable goal 3]

## Non-Goals

- [Explicitly excluded item 1]
- [Explicitly excluded item 2]

## Existing Code Context

### Project Structure
- [Relevant directories and their purposes]

### Files to Modify
- `path/to/file1.go` - [What changes]
- `path/to/file2.go` - [What changes]

### Files to Create
- `path/to/new_file.go` - [Purpose]

### Patterns to Follow
- [Pattern 1 with example file reference]
- [Pattern 2 with example file reference]

### Dependencies
- [Existing internal packages to use]
- [External libraries already in go.mod]

## Requirements

### Functional Requirements

#### X.1 [Requirement Area]
- [Specific behavior with file reference]
- [Specific behavior with file reference]

#### X.2 [Requirement Area]
- [Specific behavior with file reference]

### Non-Functional Requirements

#### Performance
- [Specific metric with number]

#### Security
- [Specific security requirement]

## Data Model

### [Table Name] (New/Modified)
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| ... | ... | ... | ... |

### Migration
- File: `migrations/000X_description.up.sql`

## API Endpoints

### [METHOD] /api/path

**Request:**
```json
{ ... }
````

**Response:**

```json
{ ... }
```

## User Journeys

### Happy Path: [Scenario]

1. [Step 1]
2. [Step 2]
3. [Step 3]

### Edge Case: [Scenario]

1. [Step 1]
2. [Step 2]

## Verification Commands

```bash
go test ./...
go build ./...
golangci-lint run
```

## Rollout Plan

1. **Phase 1**: [Core functionality]
2. **Phase 2**: [Extended functionality]

## Risks & Mitigations

### Risk: [Risk description]

**Mitigation**: [How to handle it]

## Decisions Made

- [Decision 1]: Chose X over Y because [reason]
- [Decision 2]: Chose A over B because [reason]

````

---

## Checklist Before Submitting PRD to Ralph

- [ ] All file paths are explicit (no "update the handlers")
- [ ] Existing code patterns are referenced
- [ ] Decisions are made (no "we might want to" or "TBD")
- [ ] Scope is focused (single feature, not multiple)
- [ ] Requirements are numbered and traceable
- [ ] Data model changes are complete (schema, migrations)
- [ ] API endpoints have request/response examples
- [ ] Verification commands are specified
- [ ] Non-goals clearly exclude out-of-scope items
- [ ] Performance/security requirements have specific numbers

---

## Example: Minimal Feature PRD

For small features, you can use a condensed format:

```markdown
# Add User Avatar Support - PRD

## Overview
Allow users to upload and display profile avatars.

## Goals
- Users can upload avatar images (JPEG, PNG, max 5MB)
- Avatars displayed on profile page and comments

## Non-Goals
- Image cropping/editing (out of scope)
- Multiple avatar sizes (single 200x200 for v1)

## Existing Code Context
- User model: `internal/models/user.go`
- User handlers: `internal/api/handlers/user.go`
- File upload utility: `pkg/upload/` (already handles S3)
- Image processing: add `github.com/disintegration/imaging`

## Requirements

### 8.1 Avatar Upload
- Add `POST /api/users/:id/avatar` endpoint
- Modify `internal/api/handlers/user.go`
- Validate: JPEG/PNG only, max 5MB
- Resize to 200x200 using imaging library
- Upload to S3 via existing `pkg/upload/`
- Store S3 URL in `users.avatar_url` column

### 8.2 Avatar Display
- Add `avatar_url` field to user API responses
- Modify `internal/api/handlers/user.go:GetUser`
- Return null if no avatar set

## Data Model

### users Table (Modified)
Add column:
| Column | Type | Constraints |
|--------|------|-------------|
| avatar_url | VARCHAR(500) | NULLABLE |

Migration: `migrations/000X_add_user_avatar.up.sql`

## Verification
```bash
go test ./internal/api/handlers/...
go test ./pkg/upload/...
go build ./...
````

```

---

This guide ensures your PRDs are decomposed into well-structured, executable tasks that integrate cleanly with your existing codebase.
```
