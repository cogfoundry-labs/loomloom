# Limits

- One binding per target port.
- composeValue supports string concat only.
- merge supports ordered Artifact collections and requires at least two sources.
- sequence items cannot recursively contain compose, merge, or sequence.
- stepOutput references a direct dependency and stable output port ID.
- Profile members must satisfy the Profile port contract.
- v2 has no dynamic Map/ForEach Step topology.
- Code environments and resource limits are frozen by platform Profiles; runtime dependency installation, network access, and Secret injection are prohibited.
- Loops have at most ten iterations, including the first, and a deadline of at most 1800 seconds. Initial rework support is fixed text models and Code Values.
- Conditional paths use require_all; mutually exclusive OR joins, different nested conditions, and Loop body conditions are unsupported.
- JSON Schema success does not prove target-server authorization, resolved contracts, or execution availability; server check and actual execution acceptance are required.
