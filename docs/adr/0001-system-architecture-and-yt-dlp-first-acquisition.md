# ADR-0001: System architecture and yt-dlp-first chat acquisition

- Status: Accepted
- Date: 2026-08-11
- Decision owners: Stream Analysis Tools maintainers
- Related issues: #1, #3

## Context

Stream Analysis Tools needs to register a YouTube stream, collect its archived chat in the background, persist it, and let a user browse the chat in sync with the YouTube player. Later milestones add reservations that monitor a scheduled stream and start the same collection flow after its archive becomes available.

The discarded implementation coupled product behavior to YouTube page and Innertube details. That made continuation handling, pagination, access behavior, and upstream changes application responsibilities. The replacement must establish stable component boundaries before repository scaffolding begins.

## Decision

The initial system consists of four deployment units:

| Unit | Owns | Must not own |
| --- | --- | --- |
| Web UI (React, TypeScript, Vite) | Stream registration and browsing screens, job and reservation status, chat exploration, YouTube IFrame Player integration | Direct chat-replay acquisition, yt-dlp process execution, persistence |
| Main API (Go) | Versioned HTTPS/JSON interface, input validation, stream/job/reservation orchestration, database transactions and reads | YouTube chat pagination, continuation tokens, artifact parsing, long-running collection |
| Collection Worker (Python) | Job claim/lease/heartbeat, one yt-dlp subprocess per attempt, artifact normalization, batched persistence, cancellation and cleanup | Custom Innertube or page-level HTTP client, public HTTP API, durable state outside PostgreSQL |
| PostgreSQL | Durable source of truth for streams, collection jobs and steps, chat messages, reservations and transitions | Raw temporary artifacts and process-local execution state |

The Web UI communicates with the Main API over HTTPS/JSON. The Main API and Collection Worker coordinate through PostgreSQL. The worker does not require a private API call back into the Main API.

~~~mermaid
flowchart LR
    User[User] --> Web[Web UI]
    Web -->|HTTPS / JSON| API[Go Main API]
    API --> DB[(PostgreSQL)]
    Worker[Python Collection Worker] -->|claim / lease / heartbeat| DB
    Worker --> Collector[ChatReplayCollector]
    Collector -->|one process per attempt| YtDlp[yt-dlp]
    YtDlp --> YouTube[YouTube]
    Collector -->|batch upsert| DB
    Web -.->|IFrame Player API| YouTube
~~~

### Durable state and artifacts

PostgreSQL is authoritative for:

- canonical stream identity and metadata;
- collection job and step state, progress, attempts, leases, and safe error codes;
- normalized chat messages and their stable stream-relative offsets;
- reservation state and transition history.

A yt-dlp output file is a temporary import artifact. It is not a queue, audit log, cache of record, or recovery mechanism. Each attempt uses a job-specific temporary directory. The worker removes artifacts after success, no-data results, permanent failure, timeout, or cancellation. Lease and retry state remains in PostgreSQL.

### Acquisition ownership

yt-dlp owns all YouTube chat-replay network access, including:

- discovery of the current chat replay representation;
- YouTube request construction and authentication behavior supported by yt-dlp;
- continuation-token handling;
- pagination and retry behavior inside the acquisition process;
- production of the downloaded chat artifact.

The Collection Worker owns:

- launching a pinned and checksum-verified yt-dlp version with an argument array;
- disabling ambient configuration and controlling the output path;
- enforcing time, memory, disk, and cancellation boundaries;
- terminating the subprocess tree when the attempt stops;
- stream-parsing the produced artifact without additional YouTube requests;
- normalizing supported actions, counting unsupported actions, and batch-upserting idempotently;
- redacting credentials, continuation values, chat bodies, command details, and local paths from durable errors and logs.

There is exactly one yt-dlp process per collection attempt. Worker code must not perform supplemental YouTube HTTP calls to complete or repair a replay.

### Initial collection scope

The only initial collection steps are:

1. metadata;
2. chat_replay.

Media, audio, captions, transcripts, automatic highlight extraction, and multi-stream analysis are outside M0-M4.1. Adding any of them requires an explicit issue and an architecture review of storage, lifecycle, cost, and privacy impact.

### Public interface boundary

The Main API exposes product concepts rather than yt-dlp concepts. It may accept a stream URL, start or retry a collection, and return safe progress and errors. It must not accept arbitrary yt-dlp flags, proxy settings, cookie paths, output templates, command fragments, or continuation tokens.

HTTP contracts are versioned through OpenAPI. Failures use RFC 9457 Problem Details with stable product-level codes. Internal worker identifiers, leases, filesystem paths, command lines, raw stderr, and upstream payloads do not cross the public boundary.

## Rejected alternatives

### Custom Innertube client

Rejected because it would make undocumented YouTube request formats, client identity, continuation behavior, pagination, and upstream breakage first-party application responsibilities. It duplicates the mature acquisition logic selected in yt-dlp and creates a second compatibility surface.

### Page-level chat gateway

Rejected because a gateway would preserve the same upstream coupling behind another service boundary without removing it. A separate deployment unit would add operations, security, retries, and versioning before a distinct product responsibility exists.

### Running yt-dlp in the Web UI or Main API

Rejected because collection is long-running and resource-bound. Browser execution is unsuitable, and API-process execution couples request availability to subprocess lifecycle. The worker provides explicit lease, heartbeat, retry, cancellation, and resource control.

### Raw artifacts or object storage as the source of truth

Rejected for the initial scope. Product queries require normalized, indexed, idempotent records. Temporary artifacts may be useful for a bounded diagnostic window only if a later security decision explicitly permits it; they do not replace PostgreSQL.

### Media and transcript collection in the core milestone

Rejected because neither is required to prove registration, chat collection, synchronized exploration, or reservations. Including them would expand storage, compute, copyright, privacy, and failure-mode scope before the core path is verified.

## When to reconsider an independent gateway

A gateway may be proposed only when all of the following are true:

1. A concrete, approved product requirement cannot be met by the pinned yt-dlp integration.
2. The limitation is reproduced against direct yt-dlp execution and documented with representative short, long/high-volume, unavailable, and access-restricted cases.
3. Updating, configuring, or contributing upstream to yt-dlp has been evaluated and found insufficient.
4. Measured latency, reliability, resource, or operating constraints justify ownership of YouTube network behavior.
5. The proposed gateway has an explicit owner, versioned contract, authentication and secret model, rate-limit behavior, observability, test fixtures, rollout plan, and rollback plan.
6. A separate ADR is accepted. It identifies which responsibilities move and proves there will not be two active owners for continuation handling or pagination.

A future gateway is an architectural replacement for a documented acquisition responsibility, not an extra fallback silently invoked by the worker.

## Migration from the discarded implementation

Migration is behavioral, not a source-code port:

1. Inventory user-visible behavior, representative stream cases, normalized data fields, and reusable non-sensitive fixtures.
2. Retain only product requirements and tests that do not encode Innertube requests, continuation tokens, page renderers, or gateway-specific payloads.
3. Recreate repository foundations around the four deployment units and OpenAPI/database contracts defined by this ADR.
4. Implement direct yt-dlp characterization before the collector adapter. Pin the tested version and checksum.
5. Import legacy durable data only through an explicit, validated migration into the new PostgreSQL schema. Do not migrate raw continuations, cookies, command lines, temporary files, or gateway execution state.
6. Benchmark the worker against direct yt-dlp using the same version, stream, machine, network conditions, and credentials before declaring M2 complete.
7. Cut over collection only after idempotency, lease recovery, cancellation, cleanup, redaction, and rollback checks pass.

No old component is treated as an implicit compatibility requirement. Any retained behavior must be named in an issue and verified at a product boundary.

## Consequences

### Benefits

- YouTube protocol churn is isolated behind a pinned external acquisition tool.
- Product state has one durable authority.
- API availability is separated from long-running collection.
- Worker retries are idempotent and observable at database boundaries.
- The system can replace yt-dlp later through the ChatReplayCollector contract without exposing acquisition details to the API or UI.

### Trade-offs

- Python remains a deployment runtime even though the public API is Go.
- yt-dlp upgrades require characterization, checksum updates, fixtures, and performance regression checks.
- Temporary artifacts require strict disk limits and cleanup.
- Unsupported chat actions can exist; the worker must count and surface them safely rather than silently inventing mappings.
- PostgreSQL coordination requires careful lease and transaction tests.

## Compliance checks

A change violates this ADR if it:

- adds Worker-owned YouTube chat HTTP requests;
- parses or creates continuation tokens outside yt-dlp;
- stores authoritative stream, job, message, or reservation state outside PostgreSQL;
- exposes raw yt-dlp configuration or execution details through the public API;
- adds media, audio, or transcript collection to M0-M4.1 without a new decision;
- introduces a gateway without satisfying the reconsideration criteria.

M2 completion must include a reproducible comparison with direct yt-dlp and verify one yt-dlp process per attempt, zero Worker-owned YouTube HTTP requests, bounded memory, and idempotent database writes.
