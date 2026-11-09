---
status: accepted
date: 2026-11-09
---

# Separate output completion from resource completion

An HTTP disconnect or stream error does not establish that the engine stopped computing. Use separate interfaces for generation events and release evidence, bound to an instance incarnation and execution identity. The ledger releases capacity only after matching execution completion, cancellation completion, or instance exit is confirmed. Timeouts, missing heartbeats, and expired leases leave occupancy unresolved. This can reduce availability and require managed-instance cleanup, but prevents the same capacity being reassigned while old work still runs. Backend implementations must preserve this distinction.
