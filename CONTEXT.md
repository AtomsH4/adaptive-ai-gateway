# Resource-aware inference on personal computers

This context covers local inference and its use of resources shared with everyday applications.

## Language

**Managed Engine**:
An inference service instance whose lifecycle is controlled by the gateway.
_Avoid_: attached engine, arbitrary local model service

**Attached Engine**:
An inference service instance that the gateway can call without controlling its lifecycle.
_Avoid_: managed engine, managed process

**Resource Admission**:
The decision to grant new inference work permission to execute, based on host resources and existing inference occupancy.
_Avoid_: authentication, model routing

**Draining**:
A transition in which an inference instance stops accepting new work and lets accepted work finish naturally.
_Avoid_: emergency reclamation, forced unloading

**Emergency Reclamation**:
A protective action that ends managed inference occupancy to relieve critical resource pressure, potentially interrupting active generation.
_Avoid_: draining, lossless migration, silent retry

**Available Compute Capacity**:
Compute capacity that remains available to local inference after memory safety constraints and everyday application needs are met.
_Avoid_: all physical memory, fixed CPU quota, system-wide CPU idle percentage

**Model Cold Start**:
Preparing a requested model for serving when it is not ready to handle inference.
_Avoid_: execution queuing, generation execution, background prewarming

**Execution Queuing**:
The period in which a generation request waits for resource admission or an execution opportunity, excluding a shared model cold start.
_Avoid_: model cold start, generation execution

**Validation Model**:
A fixed model baseline used to validate lifecycle and resource-control behavior. Its output quality does not represent the target application.
_Avoid_: target model, quality baseline

**Target Model**:
The model selected for an actual application task.
_Avoid_: validation model

**Observation Guard**:
A protective state that pauses new inference work while preserving existing work when required memory-pressure information cannot be assessed reliably.
_Avoid_: emergency reclamation, normal memory, missing optional metrics

**Inference Instance**:
A runtime entity with its own identity and resource ownership that prepares a model and serves subsequent generation work.
_Avoid_: model file, individual request, process ID

**Completion Evidence**:
A fact confirmed by a trusted execution side that work has ended or an inference instance has exited, used to end the corresponding resource occupancy.
_Avoid_: cancellation request, output completion, loss of contact

**Resource Signal**:
A normal, pressure, or observation-guard classification derived from resource observations and used as input to admission.
_Avoid_: admission grant, capacity reservation
