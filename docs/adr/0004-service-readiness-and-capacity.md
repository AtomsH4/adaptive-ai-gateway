---
status: accepted
date: 2026-11-09
---

# Separate service readiness from immediate inference capacity

Requiring a loaded model and an available execution slot for readiness creates a cold-start deadlock behind a load balancer: an unready instance receives no request that could load its model. Make readiness describe application initialization, command handling, shutdown, and structural faults. Report immediate execution capacity and protection reasons through scheduler diagnostics and request admission. A ready service can return a resource-related 503, so operations must track API readiness and inference availability separately. Missing required observations still block new generation.
