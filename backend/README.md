# LivePoll backend

Go 1.22+ API using Gin, MongoDB, Redis, and Gorilla WebSocket. Start MongoDB and Redis, then from this directory run `go mod tidy` and `go run ./cmd/server`.

Environment: `MONGO_URI` (default `mongodb://localhost:27017`), `MONGO_DB` (`livepoll`), `REDIS_URL` (`redis://localhost:6379/0`), `PUBLIC_ORIGIN` (`http://localhost:5173`), `COOKIE_SECURE` (`false` locally; set `true` behind HTTPS), `COOKIE_DOMAIN` optional, and `PORT` (`8080`). On production HTTPS, use same-site deployment and `COOKIE_SECURE=true`.

Routes match the frontend contract: auth signup/login return `{user}`, logout returns `{message}`, and `/auth/me` returns `{user}`. Poll create/list/detail return poll objects; result returns `{poll,totalVotes,counts,percentages}`. Votes are anonymous via a durable, HttpOnly `voter_id` cookie; Mongo's unique `{pollId,voterId}` index guarantees one vote per poll even under concurrent requests. Auth uses opaque server-side sessions in Mongo, bcrypt hashes, HttpOnly session cookie, and readable double-submit `csrf_token` required in `X-CSRF-Token` for POST operations except signup/login. Set the `Origin` exactly to `PUBLIC_ORIGIN`; cross-origin requests are rejected. WebSockets are restricted to that origin.

Redis `poll:<id>:counts` hashes receive increments only after successful Mongo inserts; event updates publish through `poll:<id>:events`. Initial WebSocket snapshots and public results are computed from Mongo, so Mongo remains authoritative after cache loss. WebSocket emits `snapshot`, `vote`, and `status` JSON events. Run checks with `go test ./...`.
