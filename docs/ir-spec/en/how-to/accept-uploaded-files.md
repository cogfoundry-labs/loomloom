# Accept uploaded files

Define an artifact Template Input with MIME and cardinality, then bind it to a target `portId` with `source=templateInput`. The public constraints must be a safe subset of the contract port.

A single Template Input may already be a collection. Use merge only when combining multiple inputs or upstream outputs.
