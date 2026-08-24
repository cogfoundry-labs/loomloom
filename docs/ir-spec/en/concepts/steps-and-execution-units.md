# Steps and execution contracts

A Step is a DAG node. v2 references either a `fixedModelContract` for one exact model contract or a `capabilityProfile` for eligible implementations of one shared port contract.

The authority contract defines ports, types, MIME, cardinality, and native mappings. The Template defines where values come from. Core resolves and freezes the complete contract; clients submit references only.
