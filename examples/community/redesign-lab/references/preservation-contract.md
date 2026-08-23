# Preservation contract

Redesign carries a trust risk greenfield building doesn't: making a site
prettier while quietly breaking it. This checklist is established as a
baseline during Analyze and re-checked during Validate. Any Fail here blocks
completion, same as a Pre-Flight Check fail.

## Fidelity levels: what's fixed versus negotiable

Not every item below is fixed at every `brand_fidelity` level
(`design-brief.md`). Three items are **fixed at every level, no exception**:
legal/consent copy, analytics event coverage (the same user actions must
still be trackable, even if the underlying IDs change), and brand name/legal
identity. Everything else scales:

| Item | Conservative (default) | Moderate | Radical |
|---|---|---|---|
| Routes and anchors | Fixed | Fixed | Negotiable, flag any change explicitly (real SEO cost) |
| Primary nav labels | Fixed | May reword, not restructure | Negotiable |
| Form field names/order | Fixed | Fixed | Fixed (breaks autofill/analytics regardless of taste) |
| Brand logo / wordmark | Untouched | May restyle (color/crop), not replace | May reinterpret, name/identity still recognizable |
| Legal / consent copy | Fixed | Fixed | Fixed |
| Analytics events | Same IDs | Same events, IDs may change if remapped | Same events, IDs may change if remapped |
| Accessibility | Not regressed | Not regressed | Not regressed |
| Links resolve | Fixed | Fixed | Fixed |
| Responsive behavior | Fixed | Fixed | Fixed |

Radical still means "the same company, reinterpreted," not "delete the
preservation contract": accessibility, link integrity, responsive behavior,
form mechanics, legal copy, and trackability are never on the table at any
level. What radical actually loosens is *visual and structural* identity
(logo treatment, IA, route naming), not user trust or working functionality.

Almost all of this is reuse, not new content: pulled directly from
`design-taste-frontend` (Leonxlnx/taste-skill) §11.C and §11.F, verified by
reading both sections directly:

## From the design authority (design-taste-frontend §11.C / §11.F)

- [ ] **Routes and anchors** unchanged: page slugs and anchor IDs stay stable
      (SEO, muscle memory, deep links).
- [ ] **Primary nav labels** unchanged.
- [ ] **Form field names and order** unchanged: renaming breaks analytics and
      browser autofill even when the visual form looks identical.
- [ ] **Brand logo / wordmark** untouched — and actually present as the real
      captured asset, not silently dropped to a text-only brand name.
      Checked mechanically: `mechanical-check.py`'s `logo-present` finding
      cross-referenced against `discover.json`'s `assets.logo` entry (from
      `capture-assets.py` at Discover). Before 2026-08 this line had nothing
      to check against — no stage ever captured a real logo, so every built
      page fell back to text by default, never by a recorded decision. If
      `assets.logo` is empty (capture genuinely found nothing), a text-only
      brand name is the correct, expected outcome, not a Fail.
- [ ] **Hero visual** (photo, background-image, or video) present when a
      real one was captured — checked the same way, mechanically:
      `hero-visual-present` cross-referenced against `discover.json`'s
      `assets.hero_visual`. This one specifically used to depend on a human
      remembering to ask for it by name; it's captured unconditionally now
      (`discover-site.md`), but the cross-check exists so a slice that
      silently drops it is still caught rather than assumed fine.
- [ ] **Legal / consent copy** untouched (privacy, terms, cookie banners).
- [ ] **Analytics events** still fire from the same button/field/section IDs
      downstream tracking depends on.
- [ ] **Accessibility not regressed**: focus states, alt text, keyboard nav,
      and contrast are at least as good as the pre-redesign baseline, not just
      "passes Pre-Flight in isolation."

## Not named by the design authority, covered instead by `webapp-testing`

- [ ] **Internal and external links still resolve**: no link that worked
      before now 404s or points at a stale anchor.
- [ ] **Responsive behavior still works** at mobile, tablet, and desktop
      widths: verified by actually resizing and re-rendering, not assumed
      from the desktop screenshot alone.

## How this is checked

1. **Baseline capture** (Analyze / `extract-design-signal.md`, self-mode): the
   items above are recorded from the *existing* site before any change is
   made: this is the contract Implement is not allowed to violate.
2. **Re-check** (Validate / `validate-design.md`): every item above is
   re-verified against the *rebuilt* site, at the level the run's
   `brand_fidelity` (`design-brief.md`) actually permits: a nav-label reword
   is a Fail at conservative, expected at moderate. A route, label, field
   name, or analytics event that changed *beyond what its fidelity level
   allows* is a Fail, not a warning. Items marked "fixed at every level" in
   the table above are never negotiable regardless of `brand_fidelity`.

Validate's question is never just "is this design good." It's "did we
redesign the site without breaking it": this file is what makes that
question checkable rather than a vibe.
