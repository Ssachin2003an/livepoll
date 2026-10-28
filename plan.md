# Live Poll — implementation and design plan

## Approved scope
Build the internship's live polling application with React, Go/Gin, MongoDB, and Redis, in separate frontend/backend folders. Owners must authenticate to create/manage polls; anonymous visitors can view and vote using public poll links. Results update over WebSockets without refresh. Include local setup, tests, Docker Compose, documentation, and honest release reporting. External publishing/deployment/video recording are attempted only when the necessary account access and services are actually available.

## Architecture and implementation
- `frontend/`: React + TypeScript + Vite single-page app; typed API client, auth context, reusable layouts/components, dashboard, create poll, public poll/results pages. Relative configurable API and WebSocket endpoints; reconnect with backoff, show connection state, and resync from snapshot.
- `backend/`: Go with Gin; `cmd/server` entry point and internal config, models, validation, Mongo repositories, Redis adapters, services, handlers, middleware, routes, and WebSocket hub. Business rules remain in services, not handlers.
- MongoDB is durable source of truth: users, polls, accepted votes and opaque expiring server-side sessions; unique normalized email and `(poll_id,voter_id)` vote indexes; poll and vote lookup indexes. Backend validates input/ownership and hashes passwords with bcrypt. Authentication uses a high-entropy, opaque token in a server-side Mongo session with expiry; cookies are HttpOnly, CSRF-protected and use `SameSite=None; Secure` for public HTTPS (plain HTTP local mode is separate). A random HttpOnly voter cookie is initialized on public poll reads before voting and only its digest is stored with the vote.
- Vote workflow: validate poll/status/option/voter, insert the vote under a unique compound index, then update Redis's per-poll count hash and publish an accepted event. Mongo insert is the acceptance point; Redis errors leave a durable outbox marker and cannot generate a false realtime success. Counter recovery takes counts and replay-protection vote IDs from one Mongo vote cursor snapshot, then seeds both Redis structures atomically under a short rebuild lock so pending retries cannot double-count votes already in that snapshot. Reconciliation retries pending outbox records; result reads remain authoritative from Mongo. Event payloads identify vote IDs; clients deduplicate, reconnect with jitter and resynchronize rather than treating Pub/Sub as durable.
- Redis holds active result counters and distributes Pub/Sub updates between backend instances; WebSocket clients subscribe to one valid public poll and receive initial snapshot plus live results. Redis/Mongo are dependencies, not frontend state.
- `docker-compose.yml`: local MongoDB and Redis; `.env.example` files contain placeholders only. The backend source is separated into `config`, `model`, `validation`, `store`, `service`, `httpapi`, `auth`, and `live` (Redis/WebSocket) packages. Root README documents API, data/index model, consistency/recovery, limitations, setup, tests, deployment steps, and truthful evidence. Tests cover validation, auth, ownership, concurrent duplicate votes, UI interactions, WebSocket client updates, and real MongoDB/Redis integration when services are available.

## Project structure

```text
frontend/                 React application and frontend tests
backend/cmd/server/       Go service entry point
backend/internal/         config, domain, validation, repositories, services, handlers, middleware, Redis and WebSocket adapters
backend/tests/            integration/support tests
docs/                     architecture and submission materials
docker-compose.yml        local MongoDB and Redis
README.md                 setup, API, architecture, testing, deployment, limitations
plan.md                   approved implementation/design decisions
TODO.md                   outcome checklist
```

## Design direction
Three visual directions considered: **signal-room editorial** (0.07), **soft paper studio** (0.04), **neon arcade terminal** (0.02). Chosen: **signal-room editorial**.

- **Design movement:** contemporary Swiss editorial layout with subtle broadcast-console cues.
- **Core principles:** legibility first; results are the hero; clear live-state feedback; calm, low-friction participation.
- **Color philosophy:** warm paper/ink neutrals build trust; vivid cobalt marks the brand and active states; mint/amber communicate live/attention without relying on color alone.
- **Layout paradigm:** left-aligned editorial landing and dashboard, with a narrow poll question column beside a broad results field; mobile collapses into a single clear sequence.
- **Signature elements:** compact waveform/live pulse; oversized typographic poll questions; thin progress tracks with precise numerical labels.
- **Interaction philosophy:** immediate acknowledgement, explicit connection/voting states, no decorative friction; keyboard and touch use are first-class.
- **Animation:** restrained 160–220 ms easing on progress bars and state transitions; reduced-motion preference disables movement; no looping decoration beyond a subtle live indicator.
- **Typography:** Manrope for display and interface headings, Inter for body/UI, with tabular numerals for counts.
- **Brand essence:** a live vote room for teams and communities, built for clarity and immediacy; **focused, dependable, energetic**.
- **Brand voice:** direct, confident microcopy. Examples: “Ask the room. See the signal.” “Live now — every vote moves the picture.”
- **Wordmark & logo:** compact custom `LIVE / POLL` wordmark with a three-bar signal mark and one cobalt terminal dot.
- **Signature brand color:** Signal Cobalt (`#405CF5`).

## Material constraints and delivery
The workspace is a fresh Manus-managed web project with no starter files; React UI is built in this project, and the Go service remains in its own directory. The internship's MongoDB and Redis requirements must be implemented as actual integrations, not replaced by the WebDev managed database. Full production deployment, public GitHub publication, and a hosted video require services/accounts and must be verified before reporting success. No real secret may enter source control. Maintain a clear distinction between implemented, locally tested, deployed, and externally blocked outcomes.
