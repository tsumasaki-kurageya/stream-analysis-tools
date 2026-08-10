# Graph Report - old_docs  (2026-08-09)

## Corpus Check
- Corpus is ~11,080 words - fits in a single context window. You may not need a graph.

## Summary
- 128 nodes · 127 edges · 20 communities (15 shown, 5 thin omitted)
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 21 edges (avg confidence: 0.89)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Community 0
- Community 1
- Community 2
- Community 3
- Community 4
- Community 5
- Community 6
- Community 7
- Community 8
- Community 9
- Community 10
- Community 11
- Community 12
- Community 13
- Community 14
- Community 15
- Community 16
- Community 17
- Community 18
- Community 19

## God Nodes (most connected - your core abstractions)
1. `YouTube Data Gateway Service` - 8 edges
2. `Reservation` - 7 edges
3. `Background Collection Job Model` - 6 edges
4. `PostgreSQL Job Coordination` - 5 edges
5. `Phase 1 Archive Collection and Synchronized Viewing` - 5 edges
6. `Reservation Lifecycle` - 5 edges
7. `Phase 1 Manual Background Collection` - 5 edges
8. `Main API Control Plane` - 4 edges
9. `Job and Step State Aggregation` - 4 edges
10. `Metadata and Chat Two-step Collection` - 4 edges

## Surprising Connections (you probably didn't know these)
- `First Product Collection Foundation` --semantically_similar_to--> `Phase 1 Archive Collection and Synchronized Viewing`  [INFERRED] [semantically similar]
  index.html → development-milestones.md
- `Background Chat Collection` --semantically_similar_to--> `Phase 1 Manual Background Collection`  [INFERRED] [semantically similar]
  m2-completion-demo.md → product-roadmap.md
- `Reservation Lifecycle` --semantically_similar_to--> `Reservation State Machine`  [INFERRED] [semantically similar]
  m4-completion-demo.md → m4-reservation-data-model.md
- `RFC 9457 Problem Details Contract` --semantically_similar_to--> `Gateway Problem Details Error Contract`  [INFERRED] [semantically similar]
  development/m1-local-architecture.md → decisions/0004-youtube-data-gateway-service.md
- `Clip Discovery Experience` --semantically_similar_to--> `Contextual Multi-view Clip Support`  [INFERRED] [semantically similar]
  index.html → development-milestones.md

## Hyperedges (group relationships)
- **PostgreSQL-backed Background Collection** — old_docs_architecture_postgresql_job_coordination, old_docs_decisions_0002_m2_background_collection_job_model_background_collection_job_model, old_docs_m2_collection_data_model_collection_job_schema, old_docs_m2_collection_data_model_collection_step_schema [EXTRACTED 1.00]
- **Media and Transcript Retirement** — old_docs_decisions_0005_discontinue_media_and_transcript_collection_media_transcript_discontinuation, old_docs_implementation_plans_remove_media_and_transcript_collection_incremental_removal_rollout, old_docs_implementation_plans_remove_media_and_transcript_collection_forward_migration, old_docs_implementation_plans_remove_media_and_transcript_collection_chat_only_sync_and_search [EXTRACTED 1.00]
- **User-visible Milestone Delivery** — old_docs_development_milestones_vertical_slice_milestones, old_docs_implementation_plans_readme_dependency_ordered_planning, old_docs_development_m1_completion_demo_m1_end_to_end_validation [INFERRED 0.85]
- **Manual Collection Acceptance Flow** — old_docs_m2_completion_demo_background_chat_collection, old_docs_m3_completion_demo_video_chat_synchronization, old_docs_product_roadmap_phase_one [INFERRED 0.95]
- **Reservation-to-Automatic-Collection Flow** — old_docs_m4_completion_demo_reservation_lifecycle, old_docs_m4_reservation_data_model_state_machine, old_docs_product_roadmap_phase_two, old_docs_railway_verification_environment_worker_restart_verification [INFERRED 0.95]
- **Sensitive Information Boundary** — old_docs_m2_completion_demo_sensitive_data_redaction, old_docs_railway_verification_environment_shared_secrets, old_docs_railway_verification_environment_secret_rotation, old_docs_youtube_data_gateway_contract_sensitive_log_redaction [INFERRED 0.85]

## Communities (20 total, 5 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.12
Nodes (19): Background Chat Collection, M2 Completion Demo, Long-Running Production Stream Demo, Sensitive Data Redaction, Reservation API Error Taxonomy, Monitoring Error Persistence, Railway Verification Environment, Railway Five-Service Deployment (+11 more)

### Community 1 - "Community 1"
Cohesion: 0.16
Nodes (14): Background Collection Job Model, Chat Message Idempotency, Claim Lease and Heartbeat, Idempotent Stream Registration, M1 Stream Data Model, Stream Record, M1 End-to-end Validation, Phase 1 Archive Collection and Synchronized Viewing (+6 more)

### Community 2 - "Community 2"
Cohesion: 0.18
Nodes (11): Elapsed Time as Common Axis, PostgreSQL Job Coordination, Python Collection Worker, YouTube Data Gateway, Elapsed Millisecond Sync Contract, Normalized Transcript Segment, Independent YouTube Data Gateway Service, PostgreSQL-only Persistence (+3 more)

### Community 3 - "Community 3"
Cohesion: 0.20
Nodes (10): Main API Control Plane, Go Modular Monolith, YouTube Stream Analyzer System Architecture, ADR-0001 M1 Stream Metadata Acquisition, Preview Revalidation on Registration, Synchronous M1 Metadata Acquisition, M1 Local Architecture, OpenAPI Contract-first Generation (+2 more)

### Community 4 - "Community 4"
Cohesion: 0.20
Nodes (10): Job and Step State Aggregation, Step-level Retry, Lease Ownership and Optimistic Locking, Reservation and Collection Responsibility Boundary, Reservation State Machine, Media and Transcript Collection Discontinuation, Metadata and Chat Two-step Collection, Chat-only Sync and Search (+2 more)

### Community 5 - "Community 5"
Cohesion: 0.25
Nodes (8): Active Reservation Uniqueness, Reservation-Linked CollectionJob, M4 Reservation Data Model, Lease and Heartbeat, Optimistic Locking, Reservation, Reservation Transition, Transactional Reservation-Job Linking

### Community 6 - "Community 6"
Cohesion: 0.25
Nodes (8): Reservation State Machine, Media and Transcript Exclusion, Phase 1 Manual Background Collection, Phase 3 Real-Time Collection, Phase 2 Scheduled Analysis, YouTube Data Gateway Contract, Gateway Service ADR, Discontinue Media and Transcript Collection ADR

### Community 7 - "Community 7"
Cohesion: 0.25
Nodes (8): Background Job Architecture, Stream Collection Domain Model, Step-Level Retry, Applications and Worker Layout, Repository Structure, Milestone-Driven Scaffolding, Repository Placement Responsibilities, Contracts Database Tests and Docs Layout

### Community 8 - "Community 8"
Cohesion: 0.29
Nodes (7): Bearer Token Rotation, Gateway HTTP and Paging Contract, Gateway Problem Details Error Contract, Configuration and Secrets, Environment Variable Configuration, Secret Management, RFC 9457 Problem Details Contract

### Community 9 - "Community 9"
Cohesion: 0.33
Nodes (7): M2 E2E Fixture, Idempotent Chat Recollection, Chat Search and Timestamp Seek, M3 Video-Chat Synchronization Demo, Production Data Acceptance, Metadata and Chat Replay Collection, Video and Chat Synchronization

### Community 10 - "Community 10"
Cohesion: 0.33
Nodes (7): M4 Production Stream Completion Demo, m4-demo-report, Reservation Lifecycle, Single Automatic Collection Job, Worker Restart Recovery, Strict M4 Demo Report, Railway Worker Restart Verification

### Community 11 - "Community 11"
Cohesion: 0.50
Nodes (4): Structured Context Database, Contextual Multi-view Clip Support, Clip Discovery Experience, YouTube Stream Analyzer Project Site

### Community 12 - "Community 12"
Cohesion: 0.67
Nodes (3): ADR-0003 M3 Multi-step Collection and Sync, ADR-0005 Discontinue Media and Transcript Collection, Media and Transcript Collection Removal Plan

### Community 13 - "Community 13"
Cohesion: 0.67
Nodes (3): ADR Lifecycle, Architecture Decision Records Guide, ADR Template

### Community 14 - "Community 14"
Cohesion: 0.67
Nodes (3): YouTube Stream Analyzer Product Roadmap, Phase 4 Clip Production UI, Clip Production Analysis Support

## Knowledge Gaps
- **50 isolated node(s):** `Structured Context Database`, `ADR-0001 M1 Stream Metadata Acquisition`, `ADR-0002 M2 Background Collection Job Model`, `ADR-0003 M3 Multi-step Collection and Sync`, `Step-level Retry` (+45 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Phase 1 Manual Background Collection` connect `Community 6` to `Community 0`, `Community 9`, `Community 7`?**
  _High betweenness centrality (0.132) - this node is a cross-community bridge._
- **Why does `Background Collection Job Model` connect `Community 1` to `Community 2`, `Community 4`?**
  _High betweenness centrality (0.085) - this node is a cross-community bridge._
- **Why does `PostgreSQL Job Coordination` connect `Community 2` to `Community 1`, `Community 3`?**
  _High betweenness centrality (0.085) - this node is a cross-community bridge._
- **Are the 2 inferred relationships involving `YouTube Data Gateway Service` (e.g. with `Gateway Real-Data Precheck` and `Web-Only Public Railway Topology`) actually correct?**
  _`YouTube Data Gateway Service` has 2 INFERRED edges - model-reasoned connections that need verification._
- **Are the 2 inferred relationships involving `Phase 1 Archive Collection and Synchronized Viewing` (e.g. with `Background Collection Job Model` and `First Product Collection Foundation`) actually correct?**
  _`Phase 1 Archive Collection and Synchronized Viewing` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Structured Context Database`, `ADR-0001 M1 Stream Metadata Acquisition`, `ADR-0002 M2 Background Collection Job Model` to the rest of the system?**
  _50 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.11695906432748537 - nodes in this community are weakly interconnected._