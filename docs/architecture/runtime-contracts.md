# State, concurrency, and port contracts

Updated 2026-11-09. The coordinator, fake backend, native adapter, and HTTP layer have implementations and tests. Real-model acceptance is recorded separately in the runtime report.

## 1. State ownership

| State axis | Authority | Example states | Other modules |
| --- | --- | --- | --- |
| Engine lifecycle | Application coordinator, informed by adapter observations | unloaded / loading / ready / draining / stopping / failed / unknown | Report observations and evidence for the matching incarnation |
| Resource protection | Application coordinator | normal / constrained / observation_guard / reclaiming / recovering | Rules produce signals; samplers produce observations |
| Execution occupancy | The same capacity ledger | reserved / dispatching / running / cancel_requested / unknown / released | HTTP disconnects carry no release authority |
| Caller response | Request use case and output boundary | waiting / streaming / succeeded / failed / disconnected | Output completion carries no capacity-release authority |

API state is a projection of these axes. A failed stream can coexist with a running engine. A resident model can coexist with an observation guard. Keep these combinations representable.

## 2. Identity and evidence

`ModelRef` identifies model ID, revision, and artifact digest. `RuntimeProfileRef` independently identifies immutable serving configuration. Changing GPU layers or threads does not change weight identity.

`InstanceRef` identifies an instance and its increasing incarnation. Production instance IDs must include a new runtime-owner identity after a gateway restart, preventing aliases between old and new instances. A PID alone is insufficient.

Logical lifecycle state and physical `InstanceObservation` use different enums. For example, unloaded means there is no instance; the coordinator interprets an observed exit according to whether that exit was expected.

`ExecutionRef` binds an execution ID to its instance. Commands receive identities before effects occur. Late output, load completion, or exit evidence from a retired incarnation cannot release current capacity.

Release requires confirmed execution completion, confirmed cancellation completion, or confirmed exit of the corresponding instance. Cancellation requests, closed sockets, read timeouts, lost heartbeats, and expired leases leave occupancy unresolved. `ReleaseEvidence.ConfirmsExecution` checks scope; the adapter and backend tests establish the evidence's validity. See [ADR-0003](../adr/0003-output-and-resource-completion.md).

## 3. Outbound port semantics

| Interface | Consumer | Completion and cancellation contract |
| --- | --- | --- |
| `ResourceSampler` | Periodic collector | Failed fields become unavailable; collection performs no business action |
| `ResourceReader` | Diagnostics and read-only use cases | Returns the latest published snapshot without a new OS probe |
| `SignalEvaluator` | Coordinator or assessment use case | Pure function with explicit time; returns no execution grant |
| `Clock` | Assessment and coordinator | Supplies explicit time; the 20 ms wake-up advances the loop while business deadlines use the injected clock |
| `ModelLifecycle` | Lifecycle use cases | A Load/Stop error or cancellation can leave an ambiguous effect; retain occupancy and reconcile |
| `Generator` | Admitted execution work | Begin can have an ambiguous outcome; observe by execution reference instead of retrying |
| `GenerationRun` | Output forwarding and supervision | Next yields semantic output; Cancel requests cancellation; AwaitRelease supplies separate release evidence |

Outbound interfaces live in `internal/application/ports/`. Inbound use cases live in `internal/application/api/`, which HTTP may import. Only the composition root connects concrete backends. Handlers receive no `ModelLifecycle`, `Generator`, or process ownership.

An unsupported contract has a false capability value. Configuration requiring it is rejected during startup or capability checks.

## 4. Concurrency and outbound work

The initial configuration has one model, one execution slot, and four unexecuted requests. Only the coordinator can convert a waiting request into a reservation.

A serial command loop owns the ledger. Blocking I/O runs in bounded work outside the loop. Control messages and output data use separate paths so output backpressure cannot stall cancellation, pressure handling, deadlines, or cleanup.

The per-request output buffer is limited to 16 events and 64 KiB. Exceeding either limit cancels the slow caller. T5 measurements must validate these initial limits.

Each operation has an identity, incarnation, and completion owner. Work proven not to have been dispatched can be withdrawn. Once a port call has an ambiguous result, an error alone cannot clear occupancy; Begin failures may require ObserveExecution or managed-instance exit.

Repeated evidence cannot release an entry twice. TTL expiry and log retention do not erase unknown occupancy. Cleanup runs in a supervision context independent of the first HTTP caller and continues after its response closes.

## 5. Shared loading and budgets

A shared load has one operation identity and one deadline. Joining, cancelling one waiter, or changing protection state never extends that deadline. When the last valid demand leaves, cancel the unused load and retain its reservation until cleanup is confirmed.

A request occupies one timing phase at a time. Waiting for an active shared load consumes its load deadline. Waiting for resources, observation recovery, a slot, or draining consumes cumulative ordinary queue time. Dispatch starts execution time. The 270-second request deadline covers all phases and survives requeuing and recovery.

When deadlines coincide, apply the request deadline first, then the shared-load deadline, then the current waiting-phase budget. Resource-release supervision can outlive the response deadline and has its own cancellation/exit budget.

## 6. Resource-policy precedence

1. Preserve the phase and deadline of reclamation already triggered by valid critical pressure.
2. Current valid critical pressure starts the emergency path. Valid noncritical evidence during grace can withdraw an interruption that has not been issued.
3. Unknown or stale required memory pressure pauses new loads and executions; active generation retains its deadline.
4. Warning pressure or reliable external contention applies ordinary protection. High owned CPU alone is insufficient.
5. Grant new work only when recovery windows, memory budgets, and slot availability are satisfied.

Missing optional GPU utilization does not trigger the observation guard. Unified CPU/GPU memory uses one capacity pool. Model file size is not peak memory, and failed observations are not normal readings.

## 7. Shutdown and crashes

Controlled shutdown first marks the service unready and rejects new commands. It then drains, cancels, and confirms owned-process exit within a bounded window. Initial planning budgets are 30 seconds of draining, 3 seconds of cooperative exit, and additional cleanup time. An outer process manager must allow more time than the application needs for cleanup.

Forced process termination cannot guarantee finally handlers or child cleanup. The in-memory prototype has no automatic crash recovery, orphan-free guarantee, or exactly-once execution guarantee.

The reliable-host stage requires a minimal ownership/operation journal and startup reconciliation using PID start identity, incarnation, and ownership. Unproven ownership is quarantined and blocks new allocation; a reused PID does not authorize termination.

## 8. Required contract scenarios

Cover concurrent admission, shared deadlines, last-waiter cancellation, separate output/release completion, retired-instance evidence, duplicate release, observation recovery, slow clients, and shutdown. Current evidence includes coordinator and real-child-process fixtures. Unsupported attached backends are rejected by configuration/capability checks. Real-model and application-impact evidence remains in the runtime report.
