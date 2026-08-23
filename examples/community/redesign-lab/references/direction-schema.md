# direction.json schema (per direction slice)

One of these per variant chosen in `generate-directions.md`, stored at
`.output/directions/<variant-name>/direction.json` alongside its screenshot.

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["variant", "skill_source", "screenshot_path"],
  "properties": {
    "variant": {
      "type": "string",
      "enum": ["current-fixed", "minimalist-ui", "industrial-brutalist-ui", "high-end-visual-design", "design-taste-frontend", "ui-craft-editorial", "ui-craft-dense-dashboard", "product-proof-saas", "operational-enterprise-ai"]
    },
    "skill_source": { "type": ["string", "null"], "description": "installed skill name that governed this slice's rules; null for the current-fixed baseline, which uses the project's own existing design instead of any skill's rules" },
    "why_chosen": { "type": "string", "description": "one line: why this variant diverges from the site's current state and the other exploratory direction(s) chosen alongside it; for current-fixed, the fixes applied instead" },
    "screenshot_path": { "type": "string", "description": "full-page screenshot, top to bottom, never a viewport-height crop, for the default colorway (colorways[0])" },
    "sections_built": { "type": "integer", "minimum": 3, "description": "hero plus at least 2 more real content sections (see generate-directions.md); a hero alone doesn't give a human enough to judge the direction by" },
    "colorways": {
      "type": "array",
      "minItems": 1,
      "maxItems": 3,
      "description": "1 palette by default (rev 7 -- see generate-directions.md 'Choosing the one colorway'), up to 3 if additional ones were requested and built ('Colorways on request'); index 0 is always the one shown at Gate 1 / scored",
      "items": {
        "type": "object",
        "required": ["label", "screenshot_path"],
        "properties": {
          "label": { "type": "string", "description": "e.g. 'Ethereal Glass', or the skill's fixed-palette name if the skill has only one" },
          "screenshot_path": { "type": "string", "description": "full-page screenshot for this colorway specifically" }
        }
      }
    }
  }
}
```

`sections_built` must be >= 3. Direction Slices doesn't produce the *final*
full page (routes/components beyond what's built here still get extended in
`implement-design.md`), but it must produce enough real, distinct sections,
rendered full-page, that a human can actually evaluate the direction rather
than choosing blind on everything a hero-only slice would have hidden.

`colorways` has exactly 1 entry by default (rev 7), regardless of whether
the chosen skill has internal archetype/vibe choices to draw from — see
generate-directions.md's "Choosing the one colorway" for how that single
entry gets picked deliberately rather than defaulted to whatever's listed
first. A 2nd or 3rd entry only appears once the human actually asks to see
another palette ("Colorways on request"); each is a re-render of the
identical structural build with different tokens, not a separate build, so
building one on request stays cheap.
