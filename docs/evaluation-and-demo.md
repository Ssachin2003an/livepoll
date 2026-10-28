# Evaluation status and demo plan

## Six-criterion evaluation (2026-10-27)

| Criterion | Status | Evidence and remaining work |
|---|---|---|
| A. Does it actually work end to end? | **Not verified** | A secured Preview browser completed signup, poll creation, sign-out, anonymous vote and display of the real backend result. The real MongoDB/Redis integration suite also verifies persistence, auth, duplicate/concurrent vote rules, owner controls and two independent WebSocket subscribers. However, the two-browser no-refresh relay has not been demonstrated in parallel browser sessions, and public production has not been verified. |
| B. Does the required stack do meaningful work? | **Not verified** | React/TypeScript build and UI tests pass; Go/Gin services handle business rules; MongoDB stores users/polls/votes/sessions with indexes; Redis count hashes, idempotency, rate limits and Pub/Sub are used. The integration suite verifies records, counts and WebSocket relay within one backend process. The required cross-instance relay has not been tested with two Go servers. |
| C. Code quality and security | **Not verified** | Separate frontend/backend folders; service separation; backend validation; bcrypt; opaque expiring sessions; owner checks; CSRF/Origin; request limits; hashed voter IDs; concurrent unique constraint; Redis rebuild/outbox are present. Unit and integration tests pass. A complete independent production security review and full network-outage test remain. |
| D. UI/UX, judgment and polish | **Pass** | Responsive React UI, empty/loading/success/error/offline states, accessible labels, result bars, connection/reconnect state and reduced motion are implemented. Ten component/unit tests and a production build pass. Desktop and 375px mobile layouts were inspected in Preview; authentic images are saved in `docs/screenshots/`. Full accessibility/performance audit remains outside this local visual pass. |
| E. Useful extras | **Pass** | Owner close/reopen and live status transitions are verified in the real-service integration suite; dashboard question search and open/closed status filtering are implemented. These local extras are useful, but do not substitute for the core public release. |
| F. Actual live link | **Fail** | No working public deployment or verified HTTPS URL exists. Docker is absent in the sandbox; production host and external MongoDB/Redis credentials still need to be authorized/provisioned. |

**Submission is not ready.** Public GitHub, public application URL, two-browser/public-flow verification, the recorded/hosted video and sending the final email remain outstanding. Authentic screenshots and manual desktop/mobile Preview inspection are complete. No links are fabricated.

## Verification completed locally

- `pnpm --filter frontend build` — passed, including TypeScript compile and Vite production bundle.
- `pnpm test` — passed: 4 test files, 10 tests for signup validation/heading, poll creation, live poll voting/rendering/events/reconnect state, results and vote-event deduplication.
- `cd backend && go test ./...` — passed for auth hashing/token generation, validation, HTTP/middleware and all backend packages.
- With local MongoDB and Redis running: `MONGO_URI=mongodb://127.0.0.1:27017 REDIS_URL=redis://127.0.0.1:6379/11 go test -tags=integration -v ./tests` — passed. It verifies signup/login/logout, normalized-email uniqueness, authenticated creation and anonymous-write rejection, public CSRF/voter-cookie setup, bad-option rejection, same-voter duplicates, eight simultaneous same-cookie submissions with exactly one accepted, Mongo persistence, Redis counts, two WebSocket subscribers receiving vote/status events, owner-only close/reopen, rejection when closed, and a simulated Redis update failure retried after a cache rebuild that includes the pending vote. It does not verify an actual network outage, two separate Go server instances, UI/browser behavior, or public deployment.

## Suggested honest 3–5 minute recording script (record only after deployment)

**0:00–0:25 — Context.** Introduce Live Poll, show the real deployed URL if one exists, and say plainly whether the recording is production or local. Do not present a local Preview as a live public app.

**0:25–1:10 — Owner flow.** Sign in/create a test account; open the dashboard; create a question with two or three distinct options; copy the actual generated share URL. Point out that only the owner may create/manage polls.

**1:10–1:55 — Separate viewers.** Open the copied URL in another browser/profile. Vote there while keeping the first results page visible. Show the other view update without refresh, connection state and duplicate rejection. Show owner close/reopen only if the public deployment is actually running.

**1:55–2:30 — Actual stack.** Explain that Go/Gin validates requests and owns business logic, MongoDB stores users/polls/votes/sessions and Redis holds live counters and distributes events through Pub/Sub; React connects via WebSocket. Show real test output/records only if visible and verified.

**2:30–3:20 — Hardest challenge (use the exact implementation truth).** “The subtle bug was a Mongo vote still marked pending for Redis when the results cache was rebuilt. If the rebuild counted that vote but left it out of Redis's deduplication set, a retry could count it twice. I fixed it by reading counts and vote IDs from one Mongo snapshot and atomically rebuilding both Redis structures under a lock; a real MongoDB/Redis integration test rebuilds while a vote is pending and verifies the retry does not overcount. I also initialize each anonymous voter cookie on the public read before accepting a vote, so simultaneous first submissions share one server-enforced identity.” Say only what the final code/tests show.

**3:20–3:50 — Honest AI disclosure.** “I used Manus AI assistance for architecture, implementation, code review and test debugging. It helped produce the React/Go code and surfaced edge cases, but generated work was not treated as evidence: I ran the UI/unit and real MongoDB/Redis integration tests and corrected the Redis rebuild/idempotency race. The public deployment and video are not claimed until they are actually completed.” Adjust to match the final repository and user's own involvement.

**3:50–4:05 — Close.** Show the verified GitHub/deployment/video links only if they exist, and state limitations or missing deliverables plainly. Keep the final recording between 3 and 5 minutes.

## External release steps

1. Configure an authorized production host that supports WebSockets and a persistent external MongoDB plus Redis/Pub/Sub service; store the connection strings as protected secrets, set the exact HTTPS origin and secure-cookie settings, and deploy the container.
2. Verify health/readiness and the signup/create/share/vote flow in two real browsers against the published URL, including Redis/Mongo persistence, reconnection and owner controls.
3. Publish a clean public GitHub repository only with authorized account access; verify public access and the exact URL.
4. Record and host an actual unlisted YouTube or publicly accessible Google Drive 3–5 minute demo based on the script above; provide honest challenge and AI-use disclosure.
5. Replace all placeholders in `docs/submission-email.md` with the three verified public links. The draft remains unsent unless the user explicitly authorizes sending.
