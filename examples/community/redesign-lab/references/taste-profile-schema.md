# taste-profile.yaml schema

Produced by `extract-design-signal.md`'s reference-mode, merging one
`senlindesign/taste-skill` Design Map + Taste DNA report per named reference
site into one combined profile. Validated against this schema before handoff
to `generate-directions.md`: a malformed profile fails this stage, not
silently propagates fields as `null` into Direction Slices.

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["references", "typography", "color", "layout", "principles"],
  "properties": {
    "references": {
      "type": "array",
      "items": { "type": "string", "format": "uri" },
      "minItems": 1
    },
    "typography": {
      "type": "object",
      "properties": {
        "families": { "type": "array", "items": { "type": "string" } },
        "scale_notes": { "type": "string" }
      }
    },
    "color": {
      "type": "object",
      "properties": {
        "palette_notes": { "type": "string" },
        "accent_strategy": { "type": "string" }
      }
    },
    "layout": {
      "type": "object",
      "properties": {
        "density": { "type": "string", "enum": ["airy", "moderate", "dense"] },
        "grid_notes": { "type": "string" }
      }
    },
    "motion": { "type": "string" },
    "hierarchy": { "type": "string" },
    "brand_character": { "type": "string" },
    "principles": {
      "type": "array",
      "description": "Trigger -> Decision -> Reason -> Evidence -> Trade-off entries from senlindesign/taste-skill, merged across all references",
      "items": {
        "type": "object",
        "required": ["principle", "trigger", "decision", "reason"],
        "properties": {
          "principle": { "type": "string" },
          "trigger": { "type": "string" },
          "decision": { "type": "string" },
          "reason": { "type": "string" },
          "evidence": { "type": "string" },
          "trade_off": { "type": "string" }
        }
      }
    }
  }
}
```

The `principles` array is the part that matters most: it's *why* a
reference site works, not what it looks like. `generate-directions.md` reads
this to inform which direction variants get picked, never to copy a
reference's appearance directly.
