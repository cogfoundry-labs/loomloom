# Design brief (per run)

Captured once at the start of a run, from whichever of the three copyable
prompts (or an equivalent user request) triggered it. Not re-asked later.

```yaml
mode: existing-site | from-references     # which pipeline
target: <path to the project being redesigned>
references: []                            # only present in from-references mode
message: >                                 # the one thing the redesign must communicate
  <captured verbatim or lightly summarized from the user's own words>
brand_fidelity: conservative | moderate | radical   # default: conservative
```

`generate-directions.md` and `implement-design.md` both read `message`:
it's the one constraint that survives every stage, independent of which
direction or variant eventually wins.

`brand_fidelity` sets how much of the current site's identity survives the
redesign, independent of which direction wins: see
`preservation-contract.md` for exactly what each level fixes versus opens
up. Ask for it explicitly if the user's own request doesn't make it clear
("keep everything about our brand, just modernize the layout" reads as
conservative; "we want people to barely recognize it as the same company"
reads as radical) rather than guessing. Default to conservative when
genuinely ambiguous: it's the reversible choice, loosening later is easy,
tightening after a radical build has already thrown things away is not.
