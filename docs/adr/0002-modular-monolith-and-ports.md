---
status: accepted
date: 2026-11-09
---

# Use a modular monolith with consumer-owned ports

The initial 8 GiB host requires low resident overhead and a path to larger deployments. Use one Go application coordinator, pure domain values, and small consumer-owned interfaces. Separate adapters handle inference processes and platform sampling. Compared with extending a complete upstream platform or starting with microservices, this keeps one authority for admission and unknown occupancy and reduces deployment dependencies. It requires custom protocol mapping and lifecycle supervision. Introduce separate processes or storage when remote workers, persistence, or tenancy require them. Enforce compile-time boundaries with the dependency checker.
