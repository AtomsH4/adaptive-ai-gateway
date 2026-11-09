---
status: accepted
date: 2026-11-08
---

# Prioritize everyday applications under critical memory pressure

Local inference shares resources with everyday applications. Waiting for generation to finish could sustain critical pressure for the full 120-second execution budget. Under critical pressure, stop new admissions and allow a bounded grace period. If recovery cannot be confirmed, cancel generation and terminate the managed engine when necessary. Report the interruption without an automatic retry. This trades completion of an individual answer for timely reclamation. Ordinary configuration changes continue to use draining; grace values require hardware validation.
