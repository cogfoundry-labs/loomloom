# Limits

- One binding per target port.
- composeValue supports string concat only.
- merge supports ordered Artifact collections and requires at least two sources.
- sequence items cannot recursively contain compose, merge, or sequence.
- stepOutput references a direct dependency and stable output port ID.
- Profile members must satisfy the Profile port contract.
- v2 has no dynamic Map/ForEach Step topology.
