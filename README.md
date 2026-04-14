# TaskFlow

A minimal but complete task management system with authentication, relational data, a REST API, and a polished UI. Users can register, log in, create projects, add tasks, and assign them to team members.

## Tech Stack

| Layer      | Technology                                                        |
| ---------- | ----------------------------------------------------------------- |
| Backend    | Go 1.25, chi router, pgx (PostgreSQL driver), golang-jwt, bcrypt  |
| Frontend   | React 18, TypeScript, Vite 5, TanStack Query, React Hook Form    |
| Styling    | Tailwind CSS 3, custom component library (shadcn/ui inspired)     |
| Database   | PostgreSQL 16, golang-migrate for schema migrations               |
| Infra      | Docker Compose, multi-stage Dockerfiles, nginx                    |

---

## Architecture Decisions

**Backend structure:** The Go backend follows a layered architecture — `handler` → `repository` — with clear separation between HTTP concerns and database access. I chose `chi` for its lightweight, idiomatic middleware chaining and stdlib compatibility. `pgx` is used over `database/sql` for its native PostgreSQL type support and connection pooling via `pgxpool`.

**Authentication:** JWT with HMAC-SHA256 signing. The secret is loaded from an environment variable (never hardcoded). Passwords are hashed with bcrypt at cost 12. Auth state on the frontend is persisted in localStorage and restored on page refresh.

**Frontend architecture:** React with TypeScript and TanStack Query for server state management. This avoids manual loading/error state tracking and provides built-in cache invalidation. React Hook Form + Zod handle form validation with strong type safety. The UI components are built from scratch, inspired by shadcn/ui patterns, using Tailwind CSS utility classes.

**Optimistic updates:** Task status changes use TanStack Query's `onMutate`/`onError`/`onSettled` pattern — the UI updates immediately and reverts on error, providing a snappy experience.

**Database design:** PostgreSQL with explicit migration files (up + down) managed by golang-migrate. Tasks cascade-delete when their parent project is deleted. A `created_by` field on tasks tracks the creator separately from the assignee, enabling proper authorization on delete.

**What I intentionally left out:** WebSocket/SSE real-time updates (time constraint), drag-and-drop reordering, and dark mode toggle. These are listed in the "What You'd Do With More Time" section below.

---

## Running Locally

Assumes you have Docker and Docker Compose installed.

```bash
git clone https://github.com/sumansahoo1/Greening-India-Assingment.git
cd taskflow
cp .env.example .env
docker compose up --build
```

Notes:

- **Postgres port**: Postgres is published on host port **5433** by default (configurable via `POSTGRES_HOST_PORT` in `.env`). This avoids collisions with a locally running Postgres on 5432.

Once all three services are healthy:

- **Frontend:** http://localhost:3000
- **API:** http://localhost:8080

---

## Running Migrations

Migrations run **automatically** when the API container starts. The Go application uses `golang-migrate` programmatically — no manual step is required.

If you need to run them manually:

```bash
docker compose exec api ./api  # migrations run on startup
```

---

## Test Credentials

Two seed users are created automatically on first startup:

```
Email:    test@example.com
Password: password123

Email:    andy@example.com
Password: password123
```

The seed also creates 15 total projects and 20 tasks per project so the UI has enough data to browse.

---

## API Reference

All endpoints return `Content-Type: application/json`. Protected endpoints require `Authorization: Bearer <token>`.

### Preferences

Per-user preferences are stored server-side.

#### GET `/me/preferences`

```json
// Response 200
{ "projects_page_size": 12, "tasks_page_size": 12 }
```

#### PATCH `/me/preferences`

```json
// Request — all fields optional
{ "projects_page_size": 24 }
```

### Authentication

#### POST `/auth/register`

```json
// Request
{ "name": "Jane Doe", "email": "jane@example.com", "password": "secret123" }

// Response 201
{ "token": "<jwt>", "user": { "id": "uuid", "name": "Jane Doe", "email": "jane@example.com" } }
```

#### POST `/auth/login`

```json
// Request
{ "email": "jane@example.com", "password": "secret123" }

// Response 200
{ "token": "<jwt>", "user": { "id": "uuid", "name": "Jane Doe", "email": "jane@example.com" } }
```

### Projects

#### GET `/projects`

Lists projects the current user owns or has tasks assigned in.

Query params: `?page=1&limit=20`

```json
// Response 200
{ "projects": [...], "total": 5, "page": 1, "limit": 20 }
```

#### POST `/projects`

```json
// Request
{ "name": "New Project", "description": "Optional description" }

// Response 201
{ "id": "uuid", "name": "New Project", "description": "...", "owner_id": "uuid", "created_at": "..." }
```

#### GET `/projects/:id`

Returns project details with all tasks embedded.

```json
// Response 200
{
  "id": "uuid", "name": "...", "description": "...", "owner_id": "uuid", "created_at": "...",
  "tasks": [{ "id": "uuid", "title": "...", "status": "todo", "priority": "high", ... }]
}
```

#### PATCH `/projects/:id`

Update name and/or description. Owner only (403 otherwise).

```json
// Request — all fields optional
{ "name": "Updated Name", "description": "Updated description" }
```

#### DELETE `/projects/:id`

Deletes project and all its tasks. Owner only. Returns `204 No Content`.

#### GET `/projects/:id/stats` (Bonus)

Returns task counts grouped by status and by assignee.

```json
// Response 200
{
  "by_status": { "todo": 3, "in_progress": 2, "done": 1 },
  "by_assignee": { "uuid": { "name": "Jane", "count": 4 }, "unassigned": { "name": "Unassigned", "count": 2 } }
}
```

### Tasks

#### GET `/projects/:id/tasks`

Query params: `?status=todo&assignee=uuid&page=1&limit=20`

```json
// Response 200
{ "tasks": [...], "total": 10, "page": 1, "limit": 20 }
```

#### POST `/projects/:id/tasks`

```json
// Request
{ "title": "Design homepage", "description": "...", "priority": "high", "assignee_id": "uuid", "due_date": "2026-04-15" }

// Response 201 — task object
```

#### PATCH `/tasks/:id`

```json
// Request — all fields optional
{ "title": "Updated", "status": "done", "priority": "low", "assignee_id": "uuid", "due_date": "2026-04-20" }

// Response 200 — updated task object
```

#### DELETE `/tasks/:id`

Project owner or task creator only. Returns `204 No Content`.

### Error Responses

```json
// 400 Validation error
{ "error": "validation failed", "fields": { "email": "is required" } }

// 401 Unauthenticated
{ "error": "missing authorization header" }

// 403 Forbidden
{ "error": "forbidden" }

// 404 Not found
{ "error": "not found" }
```

---

## What I'd Do With More Time

- **Integration tests:** Add at least 5-10 tests covering auth flow, project CRUD, and task authorization using `httptest` and a test database.
- **Dark mode:** The CSS variable system is already set up for it (`.dark` class). Would add a toggle in the navbar that persists to localStorage.
- **Drag-and-drop:** Use `@dnd-kit/core` for kanban-style task columns with drag to change status.
- **Real-time updates:** WebSocket or SSE connection for live task updates across browser tabs/users.
- **Better error handling:** More granular error types in Go (custom error types instead of string matching for duplicate key detection).
- **Rate limiting:** Add middleware to prevent brute-force login attempts.
- **Assignee picker:** Add a searchable assignee dropdown and optionally restrict choices to project members rather than all users.
- **CI/CD:** GitHub Actions pipeline for linting, testing, and building Docker images.
