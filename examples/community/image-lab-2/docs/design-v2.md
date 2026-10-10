# Image Lab 2.0 — Design (draft for review)

Status: **rev 6. Milestones M0 to M5 are implemented, plus M6 Creative Direction (skill and checks, section 5.4, items 52-54). This document was reviewed against the code on 2026-10-07 and corrected: section 10.5 lists the rules the review added, item 43 the findings. Items 48-54 (workbook thumbnails, one row = one image, Creative Direction) were added after that review and were reviewed separately (item 55).**
Date: 2026-10-08.
Scope: the MVP defined in the high-level requirements, plus the decisions made
after review. `docs/design-spec.md` still describes v0.1 and is marked superseded;
where this document differs, this one wins for Image Lab 2.

---

## 1. Purpose

> Turn a business image brief into a structured, controllable set of image
> variations, then generate the selected variations at scale.

The goal is **useful creative coverage per image**, not maximum volume. Image Lab
2 does not generate every theoretical combination; it proposes the combinations
that make sense for the intent and lets the user adjust them before any money is
spent.

### Decisions made

| Topic | Decision |
|---|---|
| Spreadsheet | **Local `.xlsx` + images folder.** No Google Sheets or Drive in the MVP. Written with XlsxWriter (native checkbox), read with openpyxl. |
| Workbook handling | **Regenerated from the ledger, never edited in place**; the previous file is kept as `.prev` (section 6.3). |
| System of record | The **experiment ledger**, `ledger.json`, is the single source of truth: experiment metadata, rows, batches (with immutable approval records) and attempts. `run.json` and `session.json` are derived exports. The workbook is an edit/view surface. |
| Model in the matrix | Model is **not** a covering dimension under `single` or `fixed:X`. Under `spread` it **is** an explicit dimension (the Advisor's top 2–3 models). Quick mode is the same rule with Model as the only dimension. |
| Build gate | **M0 is a go/no-go.** If the controls fail the blind test (section 14), fix the vocabulary and compiler wording before building anything else. |
| Row selection | A `Selected` column is the **only** criterion: checked = generate. |
| First batch | **Pairwise covering design** built with allpairspy plus our own verification and repair (section 7.3). Target about 30 rows, one image each (decision 50: a row is exactly one image). |
| Plan confirmation | Intent, Fixed vs Variable, dimensions, values and rules are confirmed **together, once** (stages 2–3). |
| Results | Experiments: workbook + images folder. The v0.1 results page stays for quick mode. A contact-sheet page (`contact-sheet.html`, M5) is built for experiments. |
| Person photos | A **consent notice** at plan confirmation; no technical restrictions. |
| Execution | CogFoundry gateway directly. **No loomloom CLI.** |
| Data formats | New data files are **JSON** (`plan.json`, `ledger.json`, `controls-catalog.json`). The existing YAML parsers are not touched. |
| v0.1 `image-lab/` | **Frozen** (critical fixes only) and replaced when Image Lab 2 ships. |
| Principles dropped from v0.1 | "never writes prompts", "no reference images", "counts only 1/2/4/8". |
| Also decided | No `Prompt override`; reference support tracked as verified / documented / unknown / unsupported. |

### Hard rules

These hold throughout the design and are acceptance criteria (section 15).

1. **Every paid execution batch requires exactly one explicit user approval,
   tied to its immutable execution snapshot.** A batch that fails right after
   approval is still one approval; a retry or a replacement model is a new batch
   with its own snapshot and approval.
2. **`run` reads only the execution snapshot**, never the workbook, and refuses
   if the snapshot or any reference file no longer matches the approved hashes.
3. **An `Unknown` outcome is never retried automatically.** The gateway has no
   idempotency key, so a timeout may have been billed.
4. **Never show a cost without a known price basis.** Known and unknown parts are
   shown separately.
5. **Never silently overwrite user-owned workbook cells.**
6. **Edit the creative parameters, not the compiled prompt.**
7. **Record every request id the moment the gateway returns it**, with an atomic
   write, so an interrupted run never loses track of paid tasks.

---

## 2. Scope

### MVP (must have)

Brief and reference-image input · Creative Planner · intent confirmation · Fixed
vs Variable dimensions · dynamic Creative Schema and Controls · valid-combination
filtering · recommended first batch · editable workbook · cost estimate ·
preflight · batch generation · per-row status and retry · images in a folder ·
results as workbook links plus the images folder.

### Deferred

Automatic optimization from previous generations · autonomous iteration ·
automatic "best image" selection · A/B testing · brand guideline system · DAM /
CMS / Shopify integrations · publishing to social platforms · Google Sheets and
Drive · strength levels on controls · per-model prompt wording variants ·
image-edit-from-winner mode · a results page for experiments (contact sheet) ·
discovering a gateway balance endpoint.

### Non-goals

A model benchmark or leaderboard. A generic image editor. Maximum-volume
generation.

---

## 3. Concepts

| Term | Meaning |
|---|---|
| **Brief** | The user's business request, free text. |
| **Reference asset** | An image the output must stay faithful to (a product, a person) or draw from. A first-class input, not text in a prompt. |
| **Creative Plan** | `plan.json`: intent, fixed vs variable dimensions, schema, controls (values and wording), constraints, references. The source of truth for *what the experiment is*. |
| **Experiment** | One brief's work, in one folder. Equals a v0.1 **session**. |
| **Row** | One **creative configuration**: parameter values + model + quantity. |
| **Sample** | One execution of a row. **A row is exactly one image.** More images of the same values are more rows with the next `Take` (r001 Take 1, r002 Take 2). A row's first attempt is `r001`; later attempts (a retry after a failure, a `--again` run, or a run after the row's values were edited) are `r001-2`, `r001-3`, …: the number **continues for a row across batches**, so a sample id and its file are never reused. The workbook shows the row's latest completed image. |
| **Matrix** | All valid rows for an experiment. |
| **Batch** | The selected rows generated together. Equals a v0.1 **round**. |
| **Ledger** | `ledger.json`, the **experiment ledger and system of record**: experiment metadata, rows, batches and attempts (section 6.4). `run.json` and `session.json` are exports derived from it. |
| **Execution snapshot** | `snapshots/<fingerprint>.json`: the exact rows, compiled prompts, models, sizes, quantities and reference hashes the user approved. The only input to `run`. |

**Unifying idea.** The v0.1 behavior ("one prompt across 2–3 models") is just an
experiment whose only varied dimension is the model. One engine serves both: the
original quick flow stays as a degenerate case (section 5.2), not a second code
path.

---

## 4. Architecture

```
            ┌────────────────────────── agent (judgment) ──────────────────────────┐
 Brief ───► │ Creative Planner: intent · fixed/variable · dimensions · values · rules│
 + refs     └───────────────────────────────┬────────────────────────────────────────┘
                                            ▼ plan.json  (user confirms)
            ┌──────────────────────── scripts (deterministic) ───────────────────────┐
            │ matrix.py     enumerate → constraints → allpairspy → verify + repair    │
            │ workbook.py   write: XlsxWriter · read (read-only): openpyxl            │
            │ compile.py    plan + row → prompt TEXT only                              │
            │ image.py      Advisor · preflight · request builder · gateway runner    │
            │ build-exploration-page.py   results page (quick mode)                    │
            └───────────────────────────────┬────────────────────────────────────────┘
                                            ▼
                 CogFoundry gateway  /tasks/generations  (text and reference input)
```

**Division of labor.** The agent does only what needs judgment: classify intent,
decide what is fixed, propose dimensions, values and wording, propose
incompatibility rules, summarize for the user. Everything that must be
reproducible, testable without spending, or safe with money is a script.

**Commands.** `plan` (stages 4–5) · `preflight` (stage 7) · `run` (stage 8) ·
`retry` · `refresh` · `resolve` (quick-mode alias of the same preflight code path)
· `check`.

**Reuse.** The Planner follows the repo's existing redesign-lab pattern: a stage
skill (`skills/plan.md`) plus a schema document with JSON Schema
(`references/plan-schema.md`). Everything in the gateway runner that worked in
v0.1 is kept (section 13).

---

## 5. Workflow

### 5.1 Stages

```
 1 CAPTURE      brief + reference images
 2 UNDERSTAND   intent · reference roles · fixed vs variable  ┐ ONE plan confirmation
 2a DIRECT      (optional) 3-10 creative directions of an open brief  │ see section 5.4
 3 SCHEMA       dimensions · values · wording · constraints   ┘ (+ person notice) → plan.json
 4 MATRIX       enumerate → filter → valid combinations        (script, command `plan`)
 5 RECOMMEND    pairwise-covering first batch · cost            ── user confirms
                writes ledger.json + experiment.xlsx (rows pre-ticked)
 6 EDIT         user edits the workbook in Excel and saves it   (Image Lab does not write it)
 7 PREFLIGHT    read workbook → validate → compile → models → cost
                                                               ── APPROVE  (the one spend gate)
 8 GENERATE     background, per sample, with live progress
 9 RESULTS      images/ · regenerated workbook · cost
                → retry failed · edit and run another batch · stop
```

**Mapping to the v0.1 steps.** PLAN = stages 1–5 · QUOTE = stage 7 · APPROVE = the
gate in stage 7 · GENERATE = stage 8 · RESULTS = stage 9. Every paid execution
batch requires **exactly one explicit approval, tied to its execution snapshot**. The plan confirmation (stages 2–3) and
the batch confirmation (stage 5) are confirmations, not spend.

| # | Who | Input | Output | Confirmation |
|---|---|---|---|---|
| 1 | agent | brief, reference files, user's words | brief text, refs copied to `refs/` | none |
| 2–3 | agent | brief, refs | intent, roles, fixed/variable, dimensions, values, wording, constraints | **one** plan confirmation; person notice if a person is pictured |
| 4–5 | `plan` command + agent | `plan.json` | valid set, first-batch rows, summary | user confirms the batch |
| 6 | user | `experiment.xlsx` | edited, saved workbook | none |
| 7 | `image.py preflight` | workbook, ledger | preflight report, `snapshots/<fingerprint>.json`, fingerprint | **spend approval** |
| 8 | `image.py run` | the approved snapshot only | images, sample status, cost | none |
| 9 | agent + scripts | ledger | results message, regenerated workbook | none |

### 5.2 Quick mode (original behavior, preserved)

When the user gives only a prompt ("generate that with Image Lab", "4 options"),
stages 2–5 collapse: the Planner does **not** run, there is no workbook, and the
plan has a single implicit variable dimension, the model. The Advisor allocates
the requested count across the best-fit models exactly as in v0.1, then the same
preflight, gate, generator and rounds apply, and the v0.1 results page is built.
Counts stay 1/2/4/8 *in quick mode only*, as a UX default, not an engine limit.

Switch to the full workflow when the user supplies reference images, asks for
controlled variation ("vary the lighting and environment"), or asks for more than
8 images. A prompt-only request skips the Planner when there are no references,
no controlled variation, and at most 8 outputs are requested.

**One engine, not two.** Quick mode creates an implicit plan whose single
dimension is the model. v0.1's allocation maps exactly onto rows: its per-model
counts never exceed 3 (count 8 is 3 + 3 + 2), so each model becomes one row per image
(`Take` 1..count). It goes through the same row snapshot, the same
fingerprint and the same `preflight` code path. In the code `quick` is the entry point; the v0.1 `resolve`
and `run --alloc` are kept unchanged as a quote helper and a legacy path that uses no ledger, snapshot or
spend guard (it prints a deprecation note and cannot be mixed with `--dir`).

### 5.3 Stage details

**1 Capture.** Take the brief; copy reference files into `refs/` (never modify
originals).

**2 Understand.** The agent picks one intent from the existing list (launch /
announcement, profile / avatar, social post, blog hero, poster / flyer,
infographic / diagram, illustration / concept art, 3D render, product / e-commerce
shot, generic) and, for reference-driven work, gives each reference one role:
`product`, `person`, `style` or `general`. The intent drives **model scoring and
default sizes only**; the experiment itself is driven by what is fixed and what
varies, so a wrong intent costs model quality, not structure, and the user can
correct it. It then shows the **Fixed vs Variable** table:

| Dimension | Status |
|---|---|
| Product identity | Fixed |
| Environment | Variable |
| Composition | Variable |
| Camera | Variable |
| Lighting | Variable |
| Style | Variable |

**3 Schema and controls.** The agent chooses only the dimensions relevant to the
intent from a recommended set (subject, environment, composition, camera,
lighting, style, pose / action, mood, brand, text / graphic, reference,
optionally aspect) and proposes 3–6 understandable values for each variable
dimension, each with the wording the compiler will use, plus incompatibility
rules. Wording comes from the controls catalog where a value exists there;
otherwise the Planner writes it. Result: `plan.json`.

**Stages 2 and 3 are presented and confirmed together** as one plan confirmation:
the user sees intent, fixed vs variable, dimensions, values, rules and, if a
reference contains a person, the notice (section 8), and can edit all of it in
one turn.

**4–5 Matrix and recommendation.** See section 7. One `plan` command produces the
valid set and the recommended batch.

**6 Edit.** The user works in `experiment.xlsx` and **saves** it. Image Lab does
not write the file in this stage (section 6.3).

**7 Preflight.** See section 9.

**8 Generate.** See section 10.

**9 Results.** Images are saved per sample, the ledger is updated, the workbook is
regenerated from the ledger, and the cost is shown. Then: retry failed rows, edit
the workbook and run another batch, or stop.

### 5.4 Creative Direction (stage 2a, optional)

The planner varies **within an idea**; Creative Direction generates **the ideas worth varying**. It runs when the user asks to explore directions or concepts, or when the
brief is creatively open-ended and the user chose "Explore creative directions" when asked once (SKILL.md step 0; explicit words always win, and "generate exactly this" stays
quick mode). Its skill is `skills/direction.md`; its output is a Direction Sheet that the plan stage turns into the same `plan.json`.

- **3 to 10 directions, default 4.** Direction 1 is the user's brief (every instruction kept, compressed where needed, exact quotes for names, copy, numbers and never-rules, with a
  coverage list; never "exactly as you wrote it" unless it is). The user's copy is invariant. The other directions keep the brief's hard constraints and contrast in medium,
  setting, light, composition, energy, density and typography.
- **Direction dimension and `batch_by`.** The directions are one plan dimension; `batch_by` splits the first batch evenly across them, each with its own covering design (item 51).
- **Look traits** (`traits`: ground, medium, layout, type, density, palette) on every direction: `check` refuses a plan in which two directions differ in fewer than three
  (word overlap, so rewording does not count) or more than two share a palette (item 53). Layout values belong to the direction and are physical and spatial.
- **One wildcard.** Only one value in the plan may relax a rule of the brief (`wildcard: true`, `relaxes: [...]`, enforced by `check`, shown in the dry run, never in the prompt).
- **One runner** described in the shared opening unless the brief fixes the cast.
- **Calibration** is one image per direction (it doubles as the check that directions look different); then the rest of the batch, each with its own quote and approval.
- Evidence: `creative-direction-t1-results.md` (a local write-up with the test images, not in the repository) (items 52-54).

---

## 6. Data and files

### 6.1 Layout

```
<out>/                         # the experiment (v0.1: "session")
  plan.json                    # Creative Plan: intent, fixed/variable, dimensions, wording, constraints, refs
  ledger.json                  # experiment ledger + system of record (metadata, rows, batches, attempts)
  snapshots/<fingerprint>.json # execution snapshots written at preflight; `run` reads only these
  experiment.xlsx              # generated view (regenerated from the ledger, never edited in place)
  experiment.prev.xlsx         # the previous version, kept on every regenerate
  refs/                        # copied reference images
  round-1/                     # a batch (v0.1: "round")
    r001-1.png  r001-2.png     # {row id}-{sample}.png
    progress.json              # live state while generating
    run.json                   # derived export (cost, models, per-image records)
  round-2/ ...
  session.json                 # derived export (batches, cumulative cost); never read back
  <slug>/index.html            # v0.1 results page, quick mode only
```

Quick mode writes the same ledger, batch folders and derived exports as
experiments, so the v0.1 page builder keeps working unchanged.

### 6.2 `plan.json` (shape)

```json
{
  "schema_version": 1,
  "brief": "Lifestyle images of this jacket for an outdoor campaign",
  "intent": "social post",
  "references": [
    {"id": "ref1", "file": "refs/jacket.jpg", "role": "product",
     "contains_person": false}
  ],
  "consent_acknowledged": null,
  "fixed":   {"product identity": "the jacket in ref1, unchanged"},
  "dimensions": {
    "environment": [
      {"value": "alpine trail", "fragment": "on a sunlit alpine trail"},
      {"value": "city street",  "fragment": "on a busy city street"},
      {"value": "cafe",         "fragment": "outside a small cafe"}
    ],
    "camera":    ["front", "3/4", "side"],
    "lighting":  ["soft daylight", "golden hour", "studio"],
    "style":     ["lifestyle", "editorial"],
    "aspect":    ["4:5", "1:1"]
  },
  "constraints": [
    {"exclude": {"environment": "cafe", "lighting": "golden hour"}}
  ],
  "model_strategy": "single"
}
```

A value is either a plain string or `{value, fragment}`. A plain string uses the
controls catalog's wording for that value, or, with no entry in either, the generic phrase "{value} as the {dimension}" (the planner dry run reports such a value as having no wording). A fragment may also
carry a **separate wording for reference runs**: `{value, fragment,
fragment_with_reference}`. M0 showed this is required: strong camera wording ("camera
on the counter pointing sharply upward") works without a reference but makes the
model tilt the products when a reference is present, while a milder phrasing keeps
them upright. This per-mode wording (text vs reference) is not deferred; per-model
wording still is. `model_strategy`:
`single` (default; the Advisor's best fit for every row), `spread` (compare
models), or `fixed:<model id>`. A row's `Model` cell overrides it.

**Model and the covering design.** Model is *not* a covering dimension under
`single` or `fixed`: the covering design runs over the creative dimensions only,
and every row gets the strategy's model. Under `spread`, Model becomes an
explicit dimension whose values are the Advisor's top 2–3 models, so every model
sees every value of the other dimensions (v0.1's quick mode is the same rule with
Model as the only dimension).

`aspect` is an **optional** dimension, included only when the use case needs it
(social portrait, e-commerce square, banner). Not every model can honor an exact
ratio; see section 11.1.

The shape is documented with a JSON Schema in `references/plan-schema.md`, the
same pattern as redesign-lab's `direction-schema.md`.

### 6.3 The workbook

Two sheets:

| Sheet | Contents |
|---|---|
| `Experiment` | one row per creative configuration (below) |
| `Read me` | how to edit, which columns Image Lab reads, what the statuses mean, and a plain-text summary of the plan |

`Experiment` columns:

| Column | Owner | Notes |
|---|---|---|
| `ID` | system, **read-only** | `r001`… stable, never reused, visible but not editable |
| `Selected` | user | native checkbox (value TRUE/FALSE); **checked = generate** |
| one column per dimension | user | change values freely; values outside the plan are accepted as custom values |
| `Model` | user | blank = per the plan's `model_strategy` (Advisor); or a catalog model. **When a run finishes, a blank cell of a generated row is filled with the model that made its latest completed sample** (after the workbook merge, so a stale workbook cannot blank it); it then acts as the row's choice for further samples and the user can clear it to hand the choice back to the Advisor. A plain `refresh` never re-fills it; `refresh --record-models` does, for older experiments. |
| `Take` | user | which image of this configuration this row is, a whole number from 1 (default 1). Rows with identical values and different Takes are separate images: r001 Take 1 and r002 Take 2 make two images. Added with `add-takes`, the plan's `takes`, or by copying a row and changing Take. **Replaces the earlier `Qty`** (a row used to hold 1–3 samples); an older workbook's Qty column is ignored with a warning, and an older ledger's qty is converted when it is loaded |
| `Reference` | user | reference id(s); blank = the plan's fixed references |
| `Prompt` | system | compiled; **read-only, recomputed at preflight, never read back** |
| `Status` | system | see section 10 |
| `Image` | system | the row's picture as an in-cell thumbnail (only when images exist and Pillow is installed); click opens the raw file, hover shows a larger preview |
| `File` | system | the row's latest image by file name, as a link to the raw file |
| `Cost` | system | actual USD |
| `Notes` | user | ignored by Image Lab |

Rules:

- **The workbook is regenerated, never edited in place.** Image Lab never opens
  the user's file for modification. Refreshing it is always: (1) read the current
  file **read-only** and merge its user-owned cells into the ledger (if it can't
  be read, stop and say so; nothing is written); (2) copy it to
  `experiment.prev.xlsx`; (3) write a fresh workbook from the ledger. User-owned
  *cells* are therefore never lost. Formatting, extra sheets or charts the user
  added are not carried over, but they remain in the `.prev` copy.
- **User-owned cells win.** They are read at preflight and at refresh and merged
  into the ledger. System-owned cells are ignored on read.
- **Selection and status are independent.** An unselected row keeps its previous
  status and is simply not in the batch. Only a deleted row becomes `Removed`.
- **Rows are identified by `ID`, not position, and `ID` is not user-editable.** A
  new row without an ID gets a new stable ID at preflight; IDs are never reused.
  If a pasted row duplicates an existing ID, the later one gets a new ID and a
  warning is shown. If a user edits an ID anyway, the new value is not recognized:
  preflight warns `ID "my-product-1" not recognized (IDs are read-only), treated as
  a new row`, the row gets a new system ID, and the original row, no longer
  present, becomes `Removed` (its history is kept).
- **Only the defined user-owned columns are interpreted.** Any other column is
  preserved as a value and ignored by the compiler. A new dimension column is
  **not** added to the Creative Plan.
- **Optional column protection.** XlsxWriter can lock the system columns (`ID`,
  `Prompt`, `Status`, `Images`, `Cost`) without a password while still allowing
  inserting rows, sorting and filtering. This is optional and not yet verified in
  Excel; it adds friction, so it is decided after the MVP workbook is tried.
- **Custom values are allowed but flagged.** A value that is not in the confirmed
  plan is accepted, compiled with the generic phrasing `"<value> as the
  <dimension>"`, and shown at preflight as `⚠ custom value "rainy night" (not in
  the plan)`; the user may proceed.
- **Excel and file locks.** Writing while Excel has the file open can fail; Image
  Lab then writes `experiment.refresh-<timestamp>.xlsx` and says so. Reading while
  it is open should work but returns the last **saved** version, so preflight
  shows when the workbook was last saved. Image Lab writes the workbook only at
  stage 5 and on refresh, never during generation; live progress goes to chat and
  `progress.json`.
- **Libraries.** XlsxWriter writes (it has `insert_checkbox`); openpyxl reads
  (read-only, values). In a test, openpyxl read the checkbox cells back as
  `True`/`False`. Excel 365 shows a checkbox; older Excel and other apps show
  TRUE/FALSE (LibreOffice rendering is untested). Typing `TRUE`, `x`, `yes` or `1`
  also counts as checked.
- **Auto filter** on the header row; the first two columns and the header are frozen.
  A ticked row hidden by a filter is still ticked and still counts as selected.
- **Thumbnails.** `Image` holds an in-cell picture (`embed_image`, Excel 365/2021, the same
  requirement as the checkboxes), so they sort and filter with their row. Each is a 192x240 JPEG
  (about 15-30 KB) whose click link opens the raw file in the batch folder. Hover shows a 768x960
  JPEG (about 245 KB) as a cell note with a picture fill: XlsxWriter cannot write that, so
  `workbook._inject_hover` patches the xlsx zip after writing (VML picture fill, a relationships
  part, media files, a jpeg content type). The note box has the picture's own aspect ratio and the cell anchor is removed, so Excel sizes
  it from the style (as openpyxl comments are sized) and not from the rows it spans: a tall wrapped
  row had stretched the picture. Any failure there leaves the unpatched workbook, which is
  valid. Cost: about 270 KB per image, so a 100-image workbook is about 27 MB. Resized JPEGs are
  cached in `.thumbs/` next to the workbook. Only files inside the experiment folder are embedded.
  Without Pillow the picture columns are absent and only the `Images` link remains. The picture
  columns are system-owned: ignored on read and not carried as user extras.

### 6.4 The ledger

`ledger.json` is the **experiment ledger and system of record**. It carries a
`schema_version` and is the only file the engine reads for state. Conceptually:

```
ledger
├── experiment    metadata: brief, intent, created, schema_version
├── rows          creative configurations, parameters, take, current status
├── batches       one record per paid execution batch (below)
└── attempts      one record per sample submission (below)
```

For the MVP this is one file; splitting it later is an implementation detail.

**Batch record.** Batch number, snapshot fingerprint, approval time, models,
estimated cost, then actual cost, start and end times. The approval fields are
written once and never changed; only the outcome fields are updated as the batch
runs. This history stays meaningful after rows are edited later.

**Attempt record.** Row id and sample number, gateway request id (when one was
returned), submit time, a hash of the request body, status, cost, seconds, file,
and any gateway error. `Unknown` is preserved here as a distinct outcome, not just
a display status: when a submit times out there is no request id to poll, so the
submit time and body hash let the user match the attempt against the console's
usage log.

- **Atomic writes.** Every write goes to a temporary file and is renamed into
  place, so a crash never leaves a half-written ledger.
- **Request ids first.** A task's gateway request id is written to the ledger as
  soon as the gateway returns it, before polling begins. Without this, a crash
  after submitting would lose track of a billed task. v0.1 only wrote `run.json`
  at the end.
- **Derived exports.** `run.json` and `session.json` are generated from the ledger
  for compatibility with the v0.1 page and for human reading. The engine never
  needs to read them to determine state; batch identity and approval history live
  in the ledger.

---

## 7. Matrix and batch recommendation

### 7.1 From plan to valid combinations

```
Cartesian product of variable dimensions
        ↓  constraints (exclude rules from plan.json)
        ↓  equivalents / duplicates removed
Valid combinations
        ↓  pairwise covering (allpairspy) → verify → repair
Recommended first batch
        ↓  user confirms (and may edit afterwards)
Selected rows
```

What is **deterministic** (script): enumeration, exclusion rules, de-duplication,
the covering design and its verification. What is **judgment** (Planner, confirmed
by the user): which dimensions matter, which values to offer, which combinations
are incompatible or unlikely to be useful, expressed as `exclude` rules. The
script never guesses "unlikely to be useful" on its own.

**Model is not part of the covering unless `model_strategy` is `spread`.** Under
`single` or `fixed:X` the covering runs over the creative dimensions and every
row gets the strategy's model. Under `spread`, Model is added as an explicit
dimension (the Advisor's top 2–3 models) so each model is covered against every
value of the other dimensions.

Constraints are **hard only**. What must never change is a Fixed dimension; what
must never combine is an `exclude` rule. Soft preferences ("prefer daytime") are
not modeled or scored: the Planner applies them by not offering those values.
Enumeration is exhaustive; a plan whose Cartesian product exceeds about a
million combinations is rejected with a request to reduce dimensions or values.

### 7.2 Batch summary shown before the workbook is created

```
Valid combinations:  1,240   (excluded: 410 by rules, 36 duplicates)
Recommended batch:   30 rows, one image each = 30 images
Estimated cost:      $4.10 known · 6 rows price unverified
Covers:              6 environments × 3 compositions × 3 lighting, product fixed
Why 30:              every pair of values appears at least once (18 needed); the
                     rest is spare for the combinations you flagged as important
```

The user confirms (or changes the target size); only then are `ledger.json` and
`experiment.xlsx` written, with the recommended rows pre-ticked.

**Batch size rule.** Target about **30 rows**, then:

- never fewer than the covering minimum (if the minimum exceeds 30, use it);
- never more than the number of valid combinations;
- never more than the optional `--max-usd` budget cap allows (**not built**: `plan` has no such cap; `--max-usd` applies at `run`, and `--target` sets the batch size);
- the user can change the target at this confirmation.

A row is one image, so the first batch is 30 images, not up to 90. The plan field `takes` (default 1) makes that many images of each ticked row (as rows with Take 1..N); leave it at 1 for a first exploration.

### 7.3 Pairwise covering design (a proxy for creative coverage)

Pick rows so that **every value of every dimension appears, and every pair of
values from two different dimensions appears at least once.** For 3 dimensions
× 3 values, 27 combinations reduce to 9 with that guarantee. For 6 × 3 × 3 = 54
combinations the lower bound is 18 rows (6 × 3 pairs), so a 30-row batch covers
every pair with spare capacity.

- **Built with allpairspy** (MIT, no dependencies, supports an exclusion filter)
  instead of a hand-written algorithm. In a test it returned exactly 18 rows for
  the 6 × 3 × 3 example, identical on repeated runs.
- **Verification and repair are ours.** In a harder test (7 dimensions of 6
  values, 40 exclusion rules) allpairspy returned 43 rows but silently left 68 of
  716 valid pairs uncovered. So after it runs, `matrix.py` computes the covered
  pairs and, for each uncovered pair that some valid combination contains, adds
  such a combination. A pair that **no** valid combination contains is reported as
  impossible, never silently dropped. That test case becomes a regression fixture.
- The same verifier reports the pairwise coverage of the **selected** rows at
  preflight (an info line), since user edits can reduce it.
- The coverage guarantee is exact; that pairwise coverage amounts to good
  *creative* coverage is only a proxy. Limits: pairs, not three-way interactions;
  says nothing about whether images look good; covers variable dimensions only.
- Spare rows go to explicit user priorities if given, otherwise to the valid
  combinations that add the most uncovered pairs, then to the most distinct.

---

## 8. Reference assets and person photos

- References are copied to `refs/` and passed to the gateway as an input image
  (`image` field; URL or base64, see section 11). A reference has one of four
  roles: `product`, `person`, `style`, `general`; the compiled prompt states the
  intended relationship, and no richer reference vocabulary is built until usage
  shows what is needed. A Fixed `product` or `person` reference adds an
  instruction to every compiled prompt ("keep the product exactly as in the
  reference").
- **Reference-capable models only** receive reference rows. The catalog records
  per model which request field carries the image and how many are allowed.
- **Person notice (MVP).** When a reference contains a person, the plan
  confirmation shows: *"This uses a photo of a person. Confirm you have their
  permission. The photo is sent to CogFoundry and the image model's provider for
  processing."* The user must confirm; `plan.json` records
  `consent_acknowledged` with a timestamp. Person-reference experiments produce
  **local results only** in the MVP. Model refusals are shown as `Blocked` and
  not retried automatically. No face or age detection is attempted.

---

## 9. Preflight

Triggered when the user says "generate". Reads the workbook (read-only), merges
user edits into the ledger, then checks:

| Check | Result on failure |
|---|---|
| selected rows exist; quantities 1–3 | row `Draft` with the reason |
| every referenced file exists and is a readable image | row `Draft` |
| values present for all variable dimensions | row `Draft` |
| combination violates an exclude rule | warning (user may keep it) |
| duplicate rows (same parameters, model, references) | warning |
| model exists in the catalog and supports references where needed | row `Draft` |
| value not in the confirmed plan | `⚠ custom value`, user may proceed |
| requested aspect ratio not exactly supported by the model | info: "requested 4:5 → 1024×1536 (2:3)" |
| reference support for the model is `documented` or `unknown` | warning (`unsupported` blocks the row) |
| price known for this model **and mode** (text vs reference) | `⚠ price unverified`, never a fake number |
| known estimate exceeds balance (only when the balance is readable) | blocks approval |
| user edited a read-only `ID` | warning: "not recognized, treated as a new row" |
| pairwise coverage of the selected rows | info line |

Chat output, then **one** approval question:

```
IMAGE LAB PREFLIGHT
Workbook saved:     14:32   (save it again and re-run if you changed it since)
Rows selected:      30      Ready: 27   Issues: 3
Images:             40      Models: 3   Reference assets: 12
Estimated cost:     $6.20 known  +  3 rows price unverified (not in the total)
Pairwise coverage:  100% of valid pairs
[Fix issues]  [Generate 27 rows]  [Stop]
```

**The execution snapshot.** The flow is:

```
preflight → snapshots/<fingerprint>.json → fingerprint shown → approval → run
```

The snapshot is the artifact being approved. It is named by its fingerprint, so
running preflight twice never leaves an orphaned file and the batch number is
assigned only at run time. It contains:

```json
{
  "fingerprint": "…",
  "created_at": "…",
  "estimated_usd_known": 6.20,
  "unverified_rows": ["r014", "r021", "r027"],
  "rows": [
    {"id": "r001", "model": "google/gemini-2.5-flash-image",
     "prompt": "…", "size": "1024x1536", "aspect_ratio": "-", "qty": 1,
     "references": [{"file": "refs/jacket.jpg", "sha256": "…"}]}
  ]
}
```

The fingerprint is a hash of the canonical snapshot body. `run --confirm
<fingerprint>` reads **only** the snapshot file, verifies that its hash equals the
fingerprint, re-hashes every reference file, and refuses on any mismatch. It never
reads the workbook, so edits made while it runs cannot change what was approved.
Edit-after-approve means a new preflight and a new snapshot. Quick mode uses the
same snapshot and algorithm over its implicit rows.

---

## 10. Generation

### 10.1 Mechanics

The generator is the v0.1 gateway runner, generalized from "N branches across 2–3
models" to "one sample per row (more images are more rows)":

- **Sample ids, not letters.** v0.1 labels branches `A`–`H` (`LABELS = "ABCDEFGH"`),
  which would fail at the ninth task. Samples are `r001-1`, `r001-2`, …, and
  files are `{id}.png`.
- Bounded concurrency (default 6, configurable) using the standard library's
  thread pool, because submitting many tasks at once produced transient
  429/500/502 errors in earlier tests.
- Same polling, download, progress file and 30–60 s chat updates as v0.1.
- Per sample: request id (written immediately), status, cost, seconds, file,
  attempts, all in the ledger.
- Resumable: `run` on an existing batch skips completed samples and resumes
  polling samples whose request id is already recorded.
- **Circuit breaker.** On an authentication error or an insufficient-balance
  rejection, stop submitting new samples and report it once, instead of failing
  every remaining row. These rejections are not charged.

### 10.2 Row status

```
Draft ──► Ready ──► Queued ──► Generating ──► Completed
  ▲         │                      │  ├─────► Partial   (some samples failed)
  └─ issues ┘                      │  ├─────► Failed
                                   │  ├─────► Blocked   (model refused / moderation)
                                   │  └─────► Unknown   (ambiguous: timeout, no response)
                      Removed (row deleted from the workbook)
```

`Draft`/`Ready` are set by preflight, and only on rows that have no history: a row that is `Failed`, `Partial` or
`Completed` keeps that status through a preflight (so declining the batch loses nothing and `retry` can still
find it); a row whose values were edited after it ran is marked `Ready` as a new configuration. Selection (`Selected`) is a separate
property and does not change status. `Unknown` is new and matters: the gateway has
no idempotency key, so after a timeout a task may or may not be billed. It is
**never retried automatically (hard rule)**; the user is told and decides, using
the attempt's recorded submit time and request-body hash to check the console's
usage log (section 6.4). `Unknown` is kept as a distinct outcome in the ledger, not
only a display status.

### 10.3 Retry

`retry` builds a new batch from rows in `Failed` or `Partial` (and `Unknown`, only
on explicit user request), asking only for the samples a row is still missing. A sample
is already accounted for, and never requested again, when it is Completed, still in
flight, Blocked (the model refused), Unknown (unless the user asked for it) or billed but
not downloaded (use `recover`, which is free). It is another execution batch, so it has
its own short preflight and approval gate, but skips stages 1–6. A gateway failure is not
charged; a **download failure is** (the image exists), which is why `recover` exists.

### 10.5 Rules added after the code review (2026-10-07)

- **One writer at a time.** `run` holds `ledger.lock` (a heartbeat; a lock silent for 5 minutes is a crash and is
  taken over). While it is held, `preflight`, `retry`, `refresh`, `recover`, `quick` and a second `run` are
  refused with `BUSY` (exit 2), because `run` rewrites the whole ledger on every update. Reading (`sheet`,
  `check`, a dry run) is always allowed. This replaces the earlier idea of editing and preflighting during a run
  (sections 5.1 and 9): edit the workbook while it runs, preflight after.
- **Ambiguous submits.** A 500, 502 or 504 on a POST, or a lost connection, may come after the gateway accepted
  the task, so the attempt is `Unknown`. Only a 429 (an explicit rate-limit rejection) is retried, once.
- **Budget guard.** Counts every billed attempt (including a failed download), and in-flight samples at the
  larger of their interim cost and their estimate. Samples with no verified price go one at a time per model
  until a real billed price is known, then use it.
- **A crashing worker** marks its attempt (`Unknown` if the submit was already sent) and the batch still
  finishes (ended_at, run.json, price observations). A gateway status of CANCELED, EXPIRED or similar is a
  failure, not a 480-second wait. The token is resolved before the ledger is touched.
- **Fingerprint** covers the rows, `estimated_usd_known` and `unverified_rows`.
- **Edited rows.** Each attempt records the row values it was made with; a row edited after it ran does not
  inherit those samples for completion or retry, and its old images keep their own labels in the workbook and
  the contact sheet.
- **User cells win.** A blank Model or Reference cell clears the field (the plan or strategy applies again), a
  blank Take is 1, and a regenerated workbook never writes the old value back. Extra columns stay under their own
  headers. Value cells are text (Excel must not turn `3/4` into a date), and no cell is a formula or a link.
- **Plans.** A dimension may not be named like a workbook column or `model`; a reference needs a text id and a
  relative path inside the plan's folder; `fixed` and `prompt` are objects of text; the intent must be one of the
  known intents; `consent_acknowledged` must be a timestamp. `plan --out` refuses a folder that already holds an
  experiment. A value counts as custom when it is not in the plan (not when it lacks wording).
- **References.** At most 20 MB and a real PNG, JPEG or WebP; the file is re-hashed at send time.
- **Downloads** are written to a temporary file, checked, then renamed, so a truncated or error-page download
  never replaces a good image. Seedream returns JPEG; `recover` re-fetches billed-but-undownloaded images free.

### 10.4 Model unavailable (503)

Kept from v0.1 and generalized: the affected rows are reported with a suggested
replacement; accepting it changes the plan, so it requires a fresh preflight.

---

## 11. Models, cost and gateway

### 11.1 Advisor and request builder (kept, extended)

The deterministic scoring from v0.1 stays: model catalog × Arena scores × intent
weights × price. Changes:

- Runs **per experiment** for `single` / `spread`, and per row only when a row
  needs something different (a reference, an override).
- Candidates for reference rows are filtered to reference-capable models.
- **Known gap:** the Arena data is text-to-image only. For reference rows the
  Advisor reuses the same intent weights as a proxy and says so in the preflight.
  Identity-preservation quality is not measured.
- Size selection per model stays as in v0.1 (`pick_size`), driven by the intent's
  preferred sizes or a row's `aspect` value. An aspect ratio is **not** a free
  parameter: only the three Gemini models have a real `aspect_ratio` field; the
  others take a fixed list of sizes, so a requested ratio maps to the nearest
  supported size. Preflight shows the requested ratio and the actual size per
  row, and the ledger records the actual size.
- **One request builder.** Request assembly (model, prompt, size, aspect,
  reference field, `watermark` only where declared) is one function, `generate.build_request`
  (the v0.1 `body_for` survives only inside the legacy `run --alloc`). `compile.py` produces only the prompt
  text.

### 11.2 Cost

- Price per image per model stays in the catalog; the observed-price cache stays,
  now keyed by **model, size and mode** (`text` or `reference`). Existing entries
  without a mode count as `text`.
- **Catalog prices go stale.** On 2026-10-07 Nano Banana was measured at about
  $0.039 per image for both text-only and reference calls (9 + 9 images), against
  the catalog's $0.0032 from 2026-09-13. So no reference surcharge was observed on
  that model (an earlier "12× for references" reading was a price change, not a
  reference effect). Reference prices for the other models are still **unmeasured**
  and shown as `⚠ price unverified`, using the existing honest-unknown behavior,
  not a guessed number. A real run's actual cost updates the observed-price cache,
  and `check` already warns when a catalog entry is older than 30 days.
- **Never present a cost without a known price basis.** The estimate is the sum
  over ready rows with a known price of `price` (one image per row); rows with an unknown price
  are counted and listed separately, never folded into one confident total. The
  wallet-safety check stays conservative for unknown rows internally, but no
  invented number is displayed. Actual cost comes from the gateway's own `cost`
  field per sample.

### 11.3 Gateway-only runtime (no loomloom CLI)

v0.1 shells out to `loomloom doctor` and `loomloom balance` and reads loomloom's
config for the token. Replacement:

| Need | Plan |
|---|---|
| Token | environment variable first (existing names), config file as fallback |
| Readiness | a no-spend gateway call or token format check |
| Balance | no gateway balance endpoint is known; **preflight does not depend on it**: it shows "balance unread, the gateway will confirm" (the existing path when the balance can't be read), and the circuit breaker (section 10.1) handles a mid-run rejection. Endpoint discovery is deferred. |

### 11.4 Reference input at the gateway

The gateway accepts a reference through the `image` field. A URL and a base64
data URI both worked on Nano Banana (the only model tested). Other models, and the
`images` array form, are unverified. The catalog records each model's reference
support as one of four states, promoted by real usage rather than a probe sweep:

| State | Meaning | Behavior |
|---|---|---|
| `verified` | a real reference call succeeded | used normally |
| `documented` | the model's page lists reference input | allowed, with a preflight warning |
| `unknown` | no information | not recommended by the Advisor; allowed only by explicit user choice |
| `unsupported` | no reference input | the row is blocked |

Today only Nano Banana is `verified`; the other models are `documented`.

---

## 12. Compatibility with Image Lab v0.1

| v0.1 capability | In Image Lab 2 |
|---|---|
| Five steps PLAN / QUOTE / APPROVE / GENERATE / RESULTS | **Kept** as the outer frame (section 5.1 mapping) |
| Intent classification, 10 intents | **Kept**, plus reference roles |
| Deterministic Advisor, Arena-based | **Kept**, per experiment / row |
| One model spread (`count` 1/2/4/8) | **Kept as quick mode** and `spread` strategy; no longer an engine limit |
| Per-model size selection | **Kept** |
| Estimate before spend, actual after | **Kept**; keyed by mode |
| Fingerprint-bound approval | **Kept**, one algorithm over a row snapshot, for both modes |
| Background progress updates | **Kept** |
| `model_unavailable` replacement | **Kept**, generalized |
| Sessions and rounds | **Kept** (experiment = session, batch = round); derived from the ledger |
| Results page, fixed address, always built | **Kept for quick mode**; experiments use the workbook, the images folder and the contact sheet (M5, built) |
| Manual "pick a favourite" | **Kept** (recorded in the ledger) |
| Branch labels `A`–`H` | **Replaced** by sample ids `r001-1` |
| v0.1 `resolve` and `run --alloc` | **Kept unchanged** as a quote helper and a legacy path (no ledger, no snapshot, no spend guard; deprecation note; cannot be combined with `--dir`). `quick` then `run --dir` is the Image Lab 2 path |
| Retry failed branches | **New** (was deferred) |
| Reference images | **New** (was out of scope) |
| "Never writes prompts" | **Dropped**: the compiler writes them |
| Counts only 1/2/4/8 | **Dropped** outside quick mode |
| loomloom CLI for readiness and balance | **Replaced** (section 11.3) |
| The `image-lab/` folder | **Frozen**, critical fixes only; replaced when Image Lab 2 ships |

---

## 13. Reuse, and code and files affected

### 13.1 Reuse from v0.1 and elsewhere

| Piece | Treatment |
|---|---|
| `_req`, `_download`, `_fingerprint`, `_png_size`, `token()` | reused, then hardened after review: a download is checked (an image, the announced length) and renamed into place atomically; `_req` never follows a redirect and the generator retries a POST only on 429; `token()` takes a key only from a CogFoundry profile and refuses control characters; `_png_size` also reads JPEG |
| `_priced` and the observed-price cache | reused; key gains a `mode` |
| `score_models`, `plan_for`, `allocate`, `pick_size`, `_shape` (the Advisor) | reused |
| `_suggest_replacement` (503 handling) | reused, generalized to rows |
| `cmd_check` pattern, model catalog, Arena scores | reused and extended |
| `cmd_run`, `_poll_all`, `_write_progress`, `_report` | adapted: row samples, bounded concurrency, request ids persisted, ledger |
| `_loomloom`, `ll`, `readiness`, `balance_usd` (loomloom CLI), `LABELS` | **removed** |
| redesign-lab stage-skill, schema-doc and pipeline-manifest pattern | followed for the Planner |
| `allpairspy` (MIT, no dependencies) | **new dependency**: pairwise covering |
| `XlsxWriter`, `openpyxl` | **new dependencies**: workbook write and read |
| `Pillow` | optional: workbook thumbnails and hover previews |

Not reused: third-party controls vocabularies and their code. They are a checklist
only, because their source prompts have unclear provenance.

### 13.2 Files

| File | Change |
|---|---|
| `scripts/image.py` (**M2 done**) | loomloom CLI use removed (token from env/config, balance "unread"); the Image Lab 2 subcommands `plan`, `preflight`, `retry`, `quick`, `refresh`, `run --dir --confirm` are dispatched to `experiment.py`/`generate.py`; `check` also checks the Image Lab 2 data files; v0.1 `resolve` and `run --alloc ... --prompt ...` kept as they were. The request builder lives in `generate.py` (`build_request`) rather than in `image.py`, because v0.1's `body_for` is nested inside the legacy `cmd_run` |
| `scripts/matrix.py` (new) | enumeration, constraints, allpairspy call, coverage verification and repair |
| `scripts/compile.py` (new) | plan + row → prompt **text** only |
| `scripts/workbook.py` (new, **M1 done**) | `write_workbook(ledger)` (XlsxWriter; model dropdown from a hidden `Lists` sheet) and `read_workbook(path)` (openpyxl, read-only) |
| `scripts/ledger.py` (new, **M1 done**) | experiment ledger: atomic save/load, row IDs, `merge_workbook` (the ID, duplicate, removed and extra-column rules) |
| `scripts/preflight.py` (new, **M1 done**) | `run_preflight`: validation, prompts, model/size/price, execution snapshot and fingerprint; reuses the v0.1 Advisor from `image.py` |
| `scripts/experiment.py` (new, **M1 + M2 done**) | `plan`, `preflight`, `retry`, `quick`, `refresh`, `run`, `check`; also reachable through `image.py` |
| `scripts/generate.py` (new, **M2 done**) | the generator: `run_batch` (snapshot verification, attempts, request ids first, `Unknown`, circuit breaker, budget guard, resume), `build_request`, a `Gateway` object so tests run offline |
| `scripts/matrix.py`, `scripts/compile.py` (**M1 done**) | as above; 199 offline tests in `tests/`, run with `python -m unittest discover -s tests` |
| `requirements.txt` (new) | allpairspy, XlsxWriter >= 3.2.2 (`insert_checkbox`), openpyxl; install with `pip install -r requirements.txt` |
| `scripts/sheet.py` (new, **M5 done**) | the experiment contact sheet |
| `scripts/planner_eval.py` (new, **M4 done**) | checks a produced plan against the planner skill's rules |
| `scripts/build-exploration-page.py` | unchanged for quick mode; person-reference experiments never publish |
| `references/controls-catalog.json` (new) | our own starter vocabulary: dimension → value → neutral wording fragment |
| `references/plan-schema.md` (new) | JSON Schema for `plan.json` and the ledger |
| `skills/plan.md` (new) | Planner stage skill |
| `references/reference-support.json` (new, **M1 done**) | per-model reference support (`image` field shape, max images, state verified / documented / unknown / unsupported, measured reference price); replaces the earlier plan to extend `model-catalog.yaml`, so its hand-written parser stays untouched |
| `references/model-catalog.yaml` | prices re-measured 2026-10-07 (Nano Banana, Nano Banana 2); no schema change |
| `SKILL.md`, `pipelines/generate.yaml`, `README.md` | rewrite for the new stages; drop the "never" principles |
| `docs/design-spec.md` | add a "superseded by design-v2.md" banner |

New data files are JSON so the hand-written YAML parsers in `image.py` are not
extended.

---

## 14. Testing without spending

Everything before stage 8 is offline and deterministic:

- `check` is extended to validate `plan.json`, the controls catalog and the model
  catalog's reference fields.
- Unit-style fixtures for the matrix (known plan → known valid set and covering
  guarantee, including the dense-exclusion case that allpairspy alone fails), the
  compiler (row → expected prompt), and the workbook (edited workbook → expected
  ledger).
- `preflight` against a fixture workbook runs with no gateway call.
- A crash-and-resume test for the ledger (kill mid-batch, resume, no lost request
  ids).
- One tiny paid smoke test per release (1 row, cheapest model).

Verified on the user's Excel: the workbook opens and saves, the native checkboxes survive a save, and edits to Qty
and the checkbox are read back. Verified only with a simulated deny-write lock (Excel was not running): reading
the workbook while it is locked, and the fallback to `experiment.refresh-<time>.xlsx`. Still **unverified**: how
LibreOffice renders the checkbox column.

The Planner and the controls are validated separately, because they cannot be
unit-tested:

- **M0 controls experiment: a hard go/no-go, before any code.** If the controls
  do not produce reliably distinguishable outcomes, the whole Planner → Schema →
  Matrix → Covering chain rests on a weak foundation, so this is tested first.
  - *Setup.* One hand-written plan, two creative dimensions (for example lighting
    and camera) with three values each, compiled by hand and generated on 2
    models. A text-only leg (about 12 images, roughly $0.1) plus a **reference
    leg** (6–9 images on Nano Banana, the only model with a measured reference
    price, roughly $0.25–0.35) to test that Fixed attributes stay fixed.
  - *Blind test.* The images are shuffled and unlabeled; a reviewer assigns each
    image the value of each dimension it was meant to show.
  - *Go criteria (proposed thresholds, to confirm).* (1) Each dimension is
    identified correctly at least **80%** of the time. (2) The differences
    correspond to the intended dimension, not to a side effect. (3) The same plan
    gives consistent distinctions on both models. (4) The compiler's wording does
    not dominate or distort the intended variation. (5) Fixed attributes (the
    referenced product) stay recognizably the same in the reference leg.
  - *No-go.* Fix the controls vocabulary and the compiler wording and run M0 again
    before building M1.
- **Planner evaluation set.** Three to five representative briefs (a product
  with a reference photo, a person-photo set, a banner with a fixed aspect, a
  prompt-only request). For each, review the proposed dimensions, values and
  exclude rules; record failures and fix the instructions or catalog.

---

## 15. MVP acceptance criteria

1. A brief plus one product photo yields a plan, confirmed once, with a Fixed vs
   Variable table.
2. The matrix excludes invalid combinations and recommends a first batch whose
   pairwise coverage is verified and repaired; impossible pairs are reported.
3. `experiment.xlsx` opens in Excel and LibreOffice, with recommended rows ticked.
4. Edits to values, quantity, model and selection are honored at preflight;
   system columns are not read back.
5. Preflight reports issues per row and a cost estimate, with unknown prices shown
   as unverified, and shows when the workbook was last saved.
6. Every paid batch requires exactly one explicit approval tied to its immutable
   execution snapshot; nothing is spent without it.
7. Each row ends in a defined status; failed rows can be retried without
   rerunning the batch; ambiguous failures are never auto-retried.
8. A prompt-only run behaves as v0.1 did (quick mode), through the same engine and
   fingerprint.
9. Person photos trigger the notice (including that the photo is sent to the
   gateway and model provider), record the acknowledgment, and produce local
   results only.
10. Image Lab never silently changes or overwrites user-owned workbook cells: the
    workbook is regenerated from the merged ledger and the previous file is kept.
11. `run` reads only the approved snapshot file, never the workbook; it refuses if
    the snapshot hash or any reference file hash no longer matches.
12. Costs are shown as a known total plus a separate count of unverified rows,
    never as one total that hides unknown prices.
13. A row is exactly one image; a first batch is one image per ticked row.
14. The ledger is written atomically and each request id is recorded before
    polling; an interrupted batch resumes without losing track of submitted tasks.
15. A batch of more than 8 images works end to end (ids, progress, results).
16. `ID` is read-only: an edited ID is flagged and never silently reassigns
    history. An `Unknown` attempt records submit time and request-body hash so it
    can be reconciled with the console's usage log.


### 15.1 Status against the criteria (2026-10-07)

| # | Status |
|---|---|
| 1 | Met by `skills/plan.md` (agent behavior; evaluated twice with fresh agents, design item 41) |
| 2 | Met (`matrix.cover` verifies and repairs; impossible pairs reported) |
| 3 | Excel verified; LibreOffice not |
| 4 | Met; a cleared Model, Reference or Take cell now clears the ledger too (review item 43) |
| 5 | Met (the report shows when the workbook was saved and notes a newer `experiment.refresh-*.xlsx` that is not read) |
| 6 | Met (fingerprint over the whole snapshot body; one run at a time per experiment) |
| 7 | Met (`Unknown` is never retried; retry asks only for missing samples) |
| 8 | Met (`quick` through the same engine); the v0.1 `run --alloc` is a separate legacy path |
| 9 | Met in code (notice, timestamped acknowledgment, `plan`/`preflight`/`run` refuse without it, pages and `--inline` refused, fail closed on an unreadable plan); the full flow has not been run with a real person photo |
| 10 | Met (the workbook is regenerated; text is written as text, never as formulas or links) |
| 11 | Met (snapshot hash, reference hash at load and again at send time) |
| 12 | Met |
| 13 | Met |
| 14 | Met (atomic writes; request id written before polling; a lock keeps other writers out) |
| 15 | Met in offline tests (8 and more samples); live runs so far were up to 8 samples |
| 16 | Met (`ID` read-only, unrecognized IDs flagged; `Unknown` records submit time and request hash) |

---

## 16. Build order

The architecture is the full design above, but it is built and validated in
stages. The MVP core is **Planner → Matrix → Excel → Preflight → Generate →
Results**; everything else supports that flow.

| Milestone | Delivers | Needs the gateway? |
|---|---|---|
| **M0** Controls experiment, **go/no-go** | Blind test of whether controls produce distinguishable, consistent outputs and keep Fixed attributes fixed (section 14). No code. **M1 does not start until M0 passes.** | yes, about $0.35–0.45 (text leg + reference leg) |
| **M1** Deterministic core | From a **hand-written** `plan.json`: matrix with allpairspy + verify/repair, compiler, workbook write/read, preflight with execution snapshot and fingerprint. Tested offline. | no |
| **M2** Generator (**done**, see items 29-33) | Row/sample runner on the ledger (ids, concurrency, request ids persisted, statuses incl. `Unknown`/`Blocked`, retry, circuit breaker); quick mode as the implicit single-dimension plan; loomloom CLI removed. | yes |
| **M3** References and results (**done**, see items 34-37) | Reference input, per-model reference states, person notice, workbook regenerate with image links. | yes |
| **M4** Planner and catalog (**done**, see items 39-41) | `skills/plan.md`, `references/plan-schema.md`, `controls-catalog.json`, planner evaluation set. | no |
| **M5** (after the MVP, **done**, see item 42) | Contact-sheet results page for experiments. | no |
| **M6** Creative Direction (**done**, see items 52-54) | `skills/direction.md`, `batch_by`, wildcard flags, look traits and takes in the plan schema, tested with blind planner runs and 99 generated images. | yes (tests) |
| **M7** Tooling the tests exposed (**mostly built**, item 57) | Built: `preflight --only` / `--one-per <dimension>` (a calibration subset), value scoping (`only_in`), a did-you-mean for misspelled catalog dimensions, a complete example plan. `image.py montage --by <dimension>` (with `--blind`). Not built: direction rules in `planner_eval.py`. | no |
| **M8** Assistant fit (**proposed**, `docs/proposal-llm-fit-advisor.md`) | `llm-advice` (L1 only): measured per-step advice on the user's own assistant; evaluations on gateway models gated by a quote and confirmation, $6 hard stop set after a pilot. | yes |

Not built in the MVP: any optimization beyond the pairwise covering algorithm,
soft-preference scoring, strength levels, richer reference semantics.

---

## 17. Decisions and open items

| # | Topic | Status |
|---|---|---|
| 1 | `Prompt override` column | **No** in the MVP; users change parameters |
| 2 | First batch | **Confirmed:** about 30 rows, floor = covering minimum, ceiling = valid combinations, optional `--max-usd`; one image per row |
| 3 | Reference verification | **Decided:** four states promoted by usage, no probe sweep |
| 4 | Libraries | **Decided:** XlsxWriter (write, native checkbox) + openpyxl (read) + allpairspy; Pillow optional |
| 5 | Prompt-only requests skip the Planner | **Rule:** no references, no controlled variation, ≤ 8 outputs |
| 6 | Gateway balance endpoint | **Deferred**; preflight does not depend on it |
| 7 | Concurrency | **Start at 6**, configurable, tune from observed 429s |
| 8 | User adds or renames workbook columns | **Decided:** carried as values, ignored; they do not become plan dimensions |
| 9 | User enters a value not in the plan | **Decided:** accepted, generic wording, flagged `⚠ custom value` |
| 10 | Aspect ratio the model can't honor exactly | **Decided:** nearest supported size, shown in preflight, recorded in the ledger |
| 11 | Workbook handling | **Decided:** regenerated from the ledger, read-only reads, previous file kept as `.prev` |
| 12 | Ledger | **Decided:** `ledger.json` is the only source; `run.json` and `session.json` are derived |
| 13 | Workbook sheets | **Decided:** `Experiment` + `Read me` (no separate `Plan` sheet) |
| 14 | Plan confirmation | **Decided:** stages 2–3 confirmed together |
| 15 | Results for experiments | **Decided:** workbook + images folder; v0.1 page for quick mode; contact sheet is M5 |
| 16 | v0.1 `image-lab/` and `design-spec.md` | **Decided:** `image-lab/` frozen; `design-spec.md` gets a superseded banner |
| 17 | New data files | **Decided:** JSON; existing YAML parsers untouched |
| 18 | `resolve` vs `preflight`, fingerprints | **Decided:** one code path, one fingerprint algorithm; `resolve` is an alias |
| 19 | Ledger name and scope | **Decided:** `ledger.json` (renamed from `rows.json`): experiment metadata, rows, batches with immutable approval records, attempts |
| 20 | Model in the covering design | **Decided:** only under `spread`, as an explicit dimension; otherwise every row gets the strategy's model |
| 21 | Execution snapshot | **Decided:** `snapshots/<fingerprint>.json`; `run` reads only it and re-hashes reference files |
| 22 | M0 | **Done: conditional go (2026-10-07).** Two rounds plus probes ($1.67 total). Lighting 27/27; product identity 9/9; camera 19/27 (70%), with one failing cell (reference-mode camera). Accepted to proceed to M1; the open wording items go to the controls catalog (see `experiments/m0/m0-plan.md` section 14) |
| 26 | Reference-mode camera reliability | **Decided:** the controls catalog marks a value's reliability per mode (for example `low angle` is `unreliable` with a reference) and preflight shows an info line when a selected row uses an unreliable value in that mode |
| 27 | Reference support data | **Decided:** lives in a new JSON file, `references/reference-support.json` (state, request field, max images, reference-mode price per model), not in `model-catalog.yaml`, so the hand-written YAML parser stays untouched |
| 28 | M1 status | **Deterministic core done (2026-10-07):** matrix + covering, compiler, workbook, ledger, preflight with snapshot/fingerprint, `experiment.py` CLI. Excel verification: see items 24 and 25 |
| 29 | M2 status | **Generator done (2026-10-07), 93 offline tests passing.** Live runs against the real gateway: 3 reference samples on Nano Banana ($0.1173 vs $0.1170 estimate); quick mode with a kill-after-submit then resume (same request ids polled, none resubmitted, $0.0130); the `--max-usd` guard holding one sample back then a resume submitting only that one ($0.0138); a retry of a (simulated-failed) sample numbered `r001-3` ($0.0065); an invalid token stopping the run with "rejected, not charged" ($0). Live testing found two bugs, both fixed with regression tests: `retry`/`preflight` crashed in a quick-mode folder (no `plan.json`), so quick rows now carry their verbatim prompt; and a paused batch left a row `Generating`, now `Partial`. Observed: Sunburst bills $0.0065 to $0.0069 per image depending on the prompt, so estimates from a single observation can be about 6% off. **Not tested live:** `Unknown` (needs a real submit timeout; fake-gateway tests only) |
| 30 | Retry | **Decided:** `retry` is a preflight over rows in `Failed` or `Partial` (`Unknown` only with `--include-unknown`), regardless of `Selected`. It asks for only the missing samples (`Qty` minus completed) and continues the numbering (`first_sample` in the snapshot, so a retry of `r001-2` is `r001-3`). It writes a normal snapshot with its own fingerprint, so it has its own approval. A row is `Completed` once its completed samples reach `Qty`, across batches. `Blocked` rows are not retried |
| 31 | Run safeguards | **Decided:** a fingerprint that already ran needs `--again` (a repeat is a new spend). Without `--max-usd` the limit is 1.25x the known estimate (at least +$0.02); if any row has no verified price there is no estimate, so `--max-usd` is required. The guard counts in-flight samples at their estimate, not only completed costs |
| 32 | Ticked rows after a run | **Default, open to change:** Image Lab does not edit your cells, so a row that finished stays ticked. The next preflight warns "already Completed and still ticked, so it will be generated again (extra cost)". Untick to skip |
| 33 | Quick mode | **Done:** `quick --prompt --intent --count [--models]` uses v0.1's allocation, writes `ledger.json` plus a snapshot (no `plan.json`, no workbook), and runs through the same `run`. The prompt is verbatim. Sample ids replace the A-H labels, so 8+ samples are fine. The v0.1 exploration page is **not** adapted yet (M5) |
| 34 | Person notice (M3) | **Enforced in code:** a plan whose reference has `contains_person` or role `person` is refused by `plan`, `preflight` (an issue per row) and `run` (re-checked against the experiment's `plan.json`, so removing the acknowledgment after approval also stops it) until `image.py acknowledge-person` records `consent_acknowledged`. The notice text is in `matrix.PERSON_NOTICE` (photo sent to CogFoundry and the model provider, results stay local). `build-exploration-page.py` refuses to build a page for such an experiment. No face or age detection |
| 35 | One reference per row (M3) | **Decided:** the Reference cell is blank (the plan's only reference), a reference id, or `none` (text-only row). Several plan references with a blank cell is an issue; several ids in one cell is an issue. Multi-image requests are unverified at the gateway, so they are not sent. Models that declare `image_is_array` get a one-element list |
| 36 | Role wording (M3) | **Done:** the default reference sentence depends on the used reference's role (product/person: keep identity or likeness and do not copy pose; style: palette, light, texture only; general: keep the main subject). An explicit `prompt.reference_prefix` in the plan always wins |
| 37 | Reference states by use (M3) | **Done, and live-verified on all 9 models (2026-10-07, one sample each, the Mighty photo, $0.4017 total).** After a real reference sample completes, the model is recorded as `verified` in `reference-support.json` with its measured price and date. **Reference calls are not priced like text calls:** OpenAI models bill about 3.4x their text price with a reference (gpt-image-2 $0.0202 vs $0.0059; Sunburst/Flare $0.0225 vs $0.0065), Gemini 3 Pro $0.1385 (catalog $0.0134 was stale), Nano Banana 2 $0.0678, Seedream lite/4.5/pro $0.0372/$0.0423/$0.0507; only Nano Banana showed no surcharge. **Verified means the gateway accepted the reference and returned an image, not that the product survived.** A separate `identity` record (a person's visual check, n=1, one product) found 6 models kept the product and 2 changed it (seedream-4.5 turned the bottle into a tube; gemini-3-pro redesigned the label and dispenser). Models marked `changed` are never auto-picked for reference rows and warn when named. Preflight also warns on duplicate rows and says model choice for reference rows uses text-to-image Arena scores as a proxy. **Not done:** a failed reference call does not demote a state to `unsupported`; identity is one product, one image per model |
| 38 | Live findings from the reference run | Seedream returns **JPEG**, not PNG: the downloader rejected a billed image as "not a PNG" (3 of 8). Fixed: downloads accept PNG/JPEG/WebP, files keep their real extension, size is read from JPEG too, and `image.py recover --dir` re-fetches billed-but-undownloaded images free from the signed URL kept in the ledger. The budget guard also did nothing for rows with no price estimate; fixed (it now stops once billed spend reaches `--max-usd`), though a sample already in flight can still push the total slightly over (the run ended at $0.4017 against a $0.40 cap) |
| 39 | Controls catalog (M4) | **Done:** `references/controls-catalog.json`, 6 dimensions (lighting, camera, composition, style, mood, environment), 34 values (6 blind-tested). Only **lighting (3) and camera (3)** are blind-tested (and `three-quarter high` is flagged `weak` with a reference, 1 of 3 in M0) (M0, one product-on-counter scene); their `validation` text records the result, including per-mode camera wording (`three-quarter high`: 30-degree wording for text, 35-degree with a reference; `low angle` reference wording milder and marked unreliable). The other 28 values are plausible starter wording marked `unvalidated`; preflight and the planner dry run say so. `aspect` is not in the catalog (a size, not wording) and is now skipped by the compiler, which used to leak it into the prompt as "16:9 as the aspect". `image.py controls` lists the vocabulary |
| 40 | Planner stage (M4) | **Done:** `skills/plan.md` (steps: capture, understand, schema and controls, write and check, one plan confirmation). Limits it enforces on the planner: 2-4 dimensions, 3-4 values (2 when binary), identity Fixed for product/person references, camera wording with a reference needs a milder variant, never acknowledge consent itself, quick mode when there is no reference, no controlled variation and at most 8 images. New no-write commands: `image.py check --plan plan.json` and `image.py plan --plan ... --dry-run` (valid combinations, first-batch size, cost estimate, grouped `Check:` lines for missing, untested and unreliable wording; works before consent so the person notice and the numbers go in one confirmation) |
| 41 | Planner evaluation set (M4) | **Built and run twice with independent agents (2026-10-07).** `tests/planner-eval/briefs.json` (5 briefs: product photo with reference, person photo, banner with aspect, prompt-only routing, six-dimension ask), `scripts/planner_eval.py` (checks a plan against the skill's rules) and golden plans. **Round 1** (5 fresh agents following the first `skills/plan.md`, answer key hidden): all 5 plans passed the checker; the agents' logs listed about 10 problems each, mostly in the tools and docs (silent `check --plan`, dry run without unverified-price count or prompt, product-only catalog wording, hardcoded "Realistic" suffix, no rule for choosing 4 of 6 dimensions, contradictory sizing rules, an unreachable 'too wide' rule). Reading the sample prompt also exposed that **Fixed text never reached a reference-mode prompt**. All fixed. **Round 2** (5 fresh agents, revised skill and tools): all 5 pass; plans visibly better (Fixed items added and disclosed, unsafe catalog values avoided, sample prompt read and corrected). Round 2 found one more real bug: Fixed text naming a reference id (\"the bottle in ref1\", copied from the skill's own example) reaches the model verbatim; now warned by the dry run, flagged by the checker, and the examples fixed. Also added: wording table, size per aspect, plan-written wording flagged as untested, `ROUTE quick` marker, quick-route handling in the skill. **Round-2 open items, now addressed:** the person notice reads correctly for a user's own photo (\"your own photo, or the permission of the person pictured\"); catalog environment wording names concrete things that must be visible (sofa and window, trees and grass, a pastel-grey backdrop) and gained `office` and `city street`; a plan value with the same name as a catalog value is reported as an override by the dry run; and requirements models follow only loosely (no text, headline space, identity) are now a first-class `visual_checks` list in the plan (up to 6), printed by the dry run, written to the workbook Read me, required by the checker for banner, product and person briefs, and checked by eye on every image after the first batch (SKILL.md step 7: the agent must look, and must not claim a check passed unless it did). **Still open, by nature:** most non-lighting and non-camera wording (including the new environment wording) is untested until someone runs a blind test, which costs money; the checker proves the skill's rules were followed, not that the dimensions are the best ones; and the new `visual_checks` rule and the revised notice have not yet been exercised by a fresh planner agent (the round-2 plans predate them and would fail the new checker rule) |
| 42 | Contact sheet (M5) | **Done:** `scripts/sheet.py`, `image.py sheet --dir D [--rows DIM] [--cols DIM] [--inline] [--out F] [--selected ids]`, and built automatically after every experiment `run` (free). One self-contained `contact-sheet.html` from the ledger and snapshots: a **pivot of any two dimensions** (default the first two; `none` for a flat grid; the model is also selectable), each cell holding that combination's images with sample id, model, cost and seconds, `not generated` for combinations that were not ticked, placeholders with the reason for failed, blocked or unknown samples, a status filter, a lightbox with the image, all parameter values, notes and the exact compiled prompt, a header with the brief, intent, Fixed items, `visual_checks`, varied dimensions, images, spend, batches and problem count, and highlighting for `--selected` picks. Images are linked by relative path; `--inline` embeds them (PNG, JPEG, WebP; refused over 16 MB). **Person experiments:** a local-only banner is shown and `--inline` is refused (item 34). **Safety:** all ledger and workbook text reaches the page as JSON with `<`, `>`, `&` escaped and is drawn with `textContent`, so a hostile cell cannot inject HTML (tested). No external resources, light/dark. Works on quick-mode folders as a flat grid by model, but the v0.1 page stays the quick-mode results page. **Checked in a real browser** on the three live Mighty images (pivot, lightbox, prompt, cost). **Not done:** thumbnails (originals are linked, so a 30-image sheet loads 30 full images), recording a pick back into the ledger (the sheet is read-only; `--selected` only highlights), and a phone-width layout check |
| 43 | Code review against this spec (2026-10-07) | **Done.** Three independent read-only reviews (generator and ledger; planner, matrix, workbook; sheet, gateway layer, docs) found about 36 problems; each was verified and fixed with a regression test (`tests/test_review.py`). Worst, with the fix: `plan --out` over an existing folder destroyed the ledger (refused now); `retry` resubmitted Unknown, Blocked and billed-but-undownloaded samples inside Partial rows, and duplicated samples a paused batch still held (a retry now asks only for what is missing); two processes could overwrite one ledger (a lock; section 10.5); a cleared Model, Reference or Qty cell was silently restored (now clears); workbook text became live formulas and links (now text); extra columns could land under the wrong header; a truncated or error-page download was accepted and could replace a good image (atomic, length-checked); the contact sheet trusted ledger file paths and could embed any local file (confined); preflight overwrote the history of rows that already ran (preserved); the fingerprint did not cover the numbers the spending limit comes from (now it does); a 500 or 502 on submit was retried although it may have been billed (now Unknown); the budget guard missed billed failures and unpriced samples (fixed); the person guards failed open on an unreadable plan (fail closed); `three-quarter high` was recommended with a reference although its own data said it barely works (flagged `weak`). Also corrected in this document: the status line, test counts, 'contact sheet later', the Excel-unverified note, the catalog value count, the `resolve` alias, plain-string wording, `plan --max-usd`, and the retry and status rules. **Known gaps left open:** the v0.1 `run --alloc` path still has none of the hard rules (deprecated, not removable without breaking v0.1 users); a pick cannot be recorded back into the ledger; `plan` has no `--max-usd` or priorities for spare rows; LibreOffice rendering is unverified |
| 44 | End-to-end test with the real gateway (2026-10-07, $0.0474 total) | Run through the real CLI like a user: `check`, `plan --dry-run`, `plan --out`, a workbook edit (untick, Qty 2, a note), `preflight`, a run with a wrong key, a real run of 3 rows / 4 images with `preflight`, `refresh`, `recover` and a second `run` all answering `BUSY` while it ran, regeneration of the workbook (status, cost, image links, the user's edits kept), a value edited after the run and regenerated as a new sample, the contact sheet, quick mode with the gallery page, the person flow up to the point of spending, a preflight full of broken rows, and `retry`/`recover` with nothing to do. The images matched their controls (eye-level vs top-down, a strong orange cast) and both `visual_checks` held. **Found and fixed:** (1) after a wrong API key nothing is billed, yet re-running the same approved snapshot was refused as a \"new spend, use --again\"; a batch rejected before any billing can now be run again, and its attempts are kept but marked `superseded` (they hold no sample number and are not shown); (2) the preflight report never named the model, now lists each model and its image count; (3) the plan summary rounded `$0.0260` to `$0.03`; (4) `retry` with nothing to do printed an empty header and `0%` coverage, now says why. **Not re-checked:** the contact sheet in the browser pane on this run (the pane refused the local address), the sheet's rendering was checked on the earlier run and its data was checked here |
| 45 | A real campaign brief through experiment mode (2026-10-07, 30 images, $1.4002) | A 7,000-character adidas collage brief, 6 dimensions (accent color, pose, framing, graphic emphasis, color blob, type treatment) x 3-4 values, a balanced pairwise batch of 30 (10/10/10 per color; framing 8/8/7/7), GPT Image 2.5 Sunburst at exactly 1024x1280 (the gateway accepted the explicit size). All 30 completed. **Cost surprise:** estimated $0.207, actual $1.4002 (6.8x): Sunburst billed $0.0302 (10 images) or $0.0549 (20 images) at this size with a ~9,800-character prompt, against the catalog's $0.0069 (a figure measured at the default size with short prompts). Cause unknown; the dearer images took longer (72 s vs 50 s), consistent with `quality=auto` choosing an effort tier per image. The guard also counted in-flight images at the estimate, so the first run submitted 12 before real prices were seen and the billed total passed the approved $0.30 cap (about $0.51); fixed: the guard now counts in-flight images at the highest real price seen, and the price cache stores the batch mean. The 18 remaining rows were then run on explicit approval ($0.8894 more). **Findings on the controls (visual read of 300 px thumbnails):** followed well: accent color 30/30, type treatment, graphic emphasis, full vs 3/4 crop, headline text (spelled correctly in 29/30; one has a stray period, \"RUN YOUR. FAST.\"); weakly followed: pose (sprint, airborne and lean look alike; few are truly airborne), the falling diagonal, and color blob placement; consistent deviations: the model adds invented slogans (\"A BRIGHTER TOMORROW\", \"FASTER FURTHER BRIGHTER YOU\") and garbles small text such as ENGINEERED FOR SPEED and, once, ADIZERO. **Not done:** `quality` is not yet a request option, so cost is not predictable; the plan has no way to say \"do not add other text\" except in the brief itself |
| 46 | The `quality` setting (after item 45) | **Done:** optional plan field `quality`, validated against each model's `quality_values` in the catalog (Sunburst and Flare: auto, low, medium, high, xhigh, max; gpt-image-2: auto to high; Gemini and Seedream: none). It is sent with every request, recorded in the snapshot (so it is part of the approved fingerprint) and on each attempt, and the observed price is keyed by it (`model|size|q=<quality>`). An explicit quality is never priced from the catalog's flat figure: the preflight shows it as unverified until one image has billed, so `run` requires `--max-usd`, and the first image sets the price for the rest. An auto-pick is limited to models that have the setting; a model without it is an issue. The dry run prints the quality and why the price is unknown. `auto` or no quality keeps the old behavior |
| 47 | Second adidas round with `quality=medium` and sharper wording (2026-10-07, 32 images, $0.6877) | Same six dimensions and the same balanced 30-row design as item 45; the wording of pose, framing (a named corner-to-corner diagonal axis), color blob and a closed list of allowed text were sharpened; `quality: medium` set. A 2-image calibration ($0.0430, the price was unknown) measured **$0.0214 and $0.0216** per image, and the 30-image batch then billed **$0.6447 against the $0.645 estimate** (per image $0.0213 to $0.0216), so an explicit quality made the cost predictable and about half the `auto` average ($0.0467). Visual read of the 32 images: invented slogans eliminated; the falling diagonal now visible (headline and track slope descend to the bottom-right); forward lean and drive-phase sprint clearer; airborne flight phase better but not in every image; blob placement differences visible for the large bleeding blob, subtle for behind vs wrapping; accent color 32/32; headline spelled correctly in all; a few images keep a faded duplicate face fragment (allowed as photographic duplication, borderline against 'no duplicate runners'). Cost of the whole adidas exploration: $1.4002 (auto, 30 images) + $0.6877 = $2.0879. Lesson recorded: an estimate must come from a measured price at the same size, prompt length and quality; use a small calibration when any of them is new |
| 48 | Workbook auto filter, thumbnails and hover (2026-10-08, user feedback) | **Done:** header auto filter; `Image 1..3` in-cell pictures with click-to-open and a hover preview (picture-filled note injected into the xlsx zip). Chosen after the user tried three prototypes (floating picture, in-cell, in-cell + hover) in Excel and all three worked. Floating pictures were rejected for not following sorted rows. Tests in `test_workbook.Thumbnails`. |
| 49 | Model column showed blank after a run (2026-10-08, user feedback) | **Done:** `ledger.record_used_model` fills a generated row's blank Model with the model of its latest completed sample, called from `finalize_rows` and from `refresh(record_models=True)` (after_run, recover, `refresh --record-models`) after the workbook merge. Tests in `test_generate`. |
| 50 | One row = one image; `Qty` becomes `Take` (2026-10-08, user request) | **Done:** a row is exactly one image. `Qty` is replaced by `Take`; further images of the same values are more rows (`add-takes --dir D --rows r001[,r005] [--count N]`, the plan field `takes` 1–3 for the ticked first-batch rows, or copy a row in the workbook and change Take). A row's first attempt is `r001` (file `r001.png`), later attempts `r001-2`, … A ticked Completed row whose values are unchanged is **skipped** at preflight with a pointer to `add-takes` (it used to be generated again); editing its values makes a new configuration that replaces the image in the sheet, the old file and ledger entry are kept. The workbook has one `Image` thumbnail and a `File` column with the file name as a link. Preflight warns when two rows have the same values, model, reference and Take. Quick mode makes one row per image. Older ledgers and workbooks are converted or ignored with a warning. The snapshot keeps an internal `qty` of 1. Tests in `test_takes`. |
| 51 | First batch for a hierarchical plan (2026-10-08, adidas three-territory brief) | **Done:** the plain pairwise covering design of the three-territory plan came out 8 / 16 / 7 rows across the territories and 20 of 31 rows in one movement, because the territory that excludes least needs the most rows to show every pair. New plan field `batch_by` (a dimension name): `matrix.cover_by` gives each of its values its own covering design and an equal share of the target; a share below a value's covering minimum keeps the rows that cover the most pairs, and the dry run says which values fall short. Tests in `test_matrix.CoverByTests`. |
| 52 | Creative Direction stage (2026-10-08) | **Done as a skill and checks** (section 5.4): routing by explicit words with one question for open-ended briefs; 3-10 directions (default 4) with the user's brief as Direction 1 under a coverage rule (verbatim was impractical for an 8.5k-character brief); the user's copy invariant; one wildcard; one runner. Proposal: `docs/proposal-creative-direction-stage.md` (rev 5). |
| 53 | Direction plan mechanics (2026-10-08) | **Done in `matrix.py`:** `wildcard`/`relaxes` flags (at most one flagged value, `relaxes` needs `wildcard`), `traits` (six keys; any two directions must differ in at least three by word overlap; at most two share a palette after ignoring the foundation words most directions share), both printed by the dry run and never in the prompt. The first string-equality version of the traits check was too easy to game and was replaced. |
| 54 | Creative Direction evidence (2026-10-08) | Four free planning tests with fresh default-model agents scored blind, then paid tests: 30 images from a plan the skill wrote from prompt 1 alone (too alike: same template in every direction), then plans B and D with the revised skill: **plan D 30 images, blind sort 30/30, five clearly different layouts**; plan B (a layout lever shared across directions) rendered three of four alike. Fix: direction-specific physical layout values, own palette per direction, one runner. Gateway spend $0.9902 (all image generation, each batch quoted and approved). Details `creative-direction-t1-results.md` (a local write-up with the test images, not in the repository). |
| 55 | Review of items 48-54 (2026-10-08) | Three read-only reviews (workbook; takes, ledger and preflight; matrix and checks) found 30 issues; **all but the ones listed under "not fixed" are fixed and have regression tests (296 tests).** *Billing safety:* a plain preflight now decides by what already exists for a row's values (an image, a sample in flight, a refusal, an Unknown outcome, an image billed but not downloaded), not by its status, so a row restored from Removed or edited back is never paid for twice; the "billed, not downloaded" case points to `recover`. *One writer:* preflight, refresh, recover, quick and add-takes now hold the ledger lock (re-entrant in one process), an unreadable or empty lock counts as held while recent, and the heartbeat is written atomically. *Data loss:* rows added while the workbook fell back to a side copy are no longer marked Removed by the next merge (the ledger records which rows the main workbook holds), nor are take rows added by the legacy Qty migration; a renamed or deleted dimension column keeps the ledger's values and warns; a date in an extra column no longer breaks the ledger save; the Model write-back only covers the rows of the batch that finished, so a cleared cell stays cleared. *Workbook:* a corrupt thumbnail cache is detected and regenerated; the main workbook is written to a temp file and renamed; control characters, transparency and EXIF orientation are handled; the newest readable image is shown; thumbnails and previews come from one decode. *Matrix:* the palette rule no longer rejects three different palettes that overlap pairwise and now catches chains; trait comparison keeps size and number words, handles any script and treats empty text strictly; `validate_plan` returns errors instead of crashing; `cover_by` reports an empty value and gives its share to the others, handles a target of zero, and enumerates once; a dry-run warning appears when `batch_by` has no traits. Two older tests had their exclusions in the wrong format and never exercised them; fixed. **Not fixed (accepted or unverifiable):** a stale main workbook can still blank a recorded Model after a fallback (the CLI now says when it fell back); workbook size is about 270 KB per image (the hover preview); `#` in an image file name breaks its link (generated names never contain it); row heights after a user sort and the hover note in real Excel remain unverified here; the traits check is word-overlap only and cannot tell "left" from "right", so the generated images stay the real test; the minimum of three directions and the wildcard-from-four rule are guidance in the skill, not enforced by code. |
| 56 | Assistant fit (proposal, 2026-10-08) | **Decided, not built:** L1 advice only; evidence from our own measured evaluations first; a model is used either as the coding agent's default (no gateway charge) or through the gateway with a quote and your confirmation; any other agent-side model only if you select it per run; $6 hard stop set after a pilot; Fable 5 and GPT-6 Astra gateway-only; "Assistant fit" in text, `llm-advice` in code. |
| 57 | Fixes from the cold-start test (2026-10-08) | A fresh agent given only the README and skills reached a valid preflight with no spend. Fixed: `preflight --only r001,r009` and `--one-per direction` price a subset of the ticked rows (the rest stay ticked), which is the documented calibration route; `only_in` on a value replaces up to dozens of hand-written exclude pairs; `references/examples/direction-plan.json` is a complete, tested plan; the worked example in `skills/direction.md` no longer breaks its own keep-out rule; SKILL.md rule numbering and routing; README now leads with Image Lab 2; preflight says to pass `--max-usd` when prices are unverified. **Not fixed:** `check` cannot catch prose contradictions; "no X" keep-outs may be drawn by a model (the visual checks are the only signal). |
| 58 | Second cold-start test (2026-10-08) | A fresh agent with only the merged README and SKILL.md reached a valid one-image-per-direction preflight in about 12 tool calls with nothing blocking it. Fixed: preflight prints an **Indicative** price range and a safe `--max-usd` from `references/price-hints.json` when no price has been observed (a hint, never part of a quote or the fingerprint; only for text rows with an explicit quality, never for reference or auto-quality rows, and ignored after 180 days); the coverage line says it counts all ticked rows when the batch is a subset; the dry run explains why a target of 30 becomes 32; the v0.1 README moved to `docs/v0.1-readme.md`; the token is described as needed only for spending commands; direction.md's sportswear wording is generic; montage and PowerShell notes. **Not fixed:** reading SKILL.md, direction.md, plan.md and plan-schema.md before writing a first plan is most of the 6 to 8 minutes, and the docs still give no way to skip the parts that do not apply to a brief. |
| 59 | Shorter first read (2026-10-09) | A third cold-start test (same brief as the second) with a start-here box in SKILL.md, a one-page `references/quickstart.md`, a second complete example (`variation-plan.json`) and the quick-mode and after-approval steps moved to `skills/quick.md` and `skills/results.md`: the agent read about 30 KB of docs instead of about 62 KB (4 files to 4, the long ones skipped), made 11 tool calls instead of 13, used about 24% fewer tokens, and `check` passed first time. `skills/plan.md` and `skills/direction.md` were deliberately left unchanged because the advisor's measured results (Creative Direction, plan writing) embed their text. Found and fixed: the planner's limits (4 variable dimensions, about 150 combinations, 4 values) were only in the skill text and the evaluation script, so the dry run now warns about them (creative directions exempt); a long-prompt warning no longer shows for a single auto-picked model; the quickstart says to list every piece of text, and that a shared lever must suit every direction. **Not fixed:** `check` cannot see a shared lever that contradicts a direction. |
| 60 | Routing by intent and constraints (2026-10-10) | Owner feedback: the three routes answer three different questions (Quick: the same prompt, fast; Variation: which variables matter; Creative Direction: what different ideas can this brief become) and the keyword rule ("explore", "different X", a photo) was too weak. SKILL.md step 0 and the quickstart now give one decision order, first match wins: explore creative directions -> Creative Direction; control variables, or a need Quick cannot meet (a reference photo, more than 8 images) -> Variation; an open-ended brief with no stated intent -> ask once; otherwise Quick. The Quick approval message now says Quick varies the model and seed, not the composition. A 12-request routing set (`tests/planner-eval/route-cases.json`, `planner_eval.py --routes`) was run by fresh default-model agents twice on the old and the new text: old text 11 of 12 both runs (it sent "8 different product shots" to Variation), new text 12 of 12 both runs. Small set, written by the author, default model only. `skills/plan.md` still says "the user's words decide"; it was left unchanged because the plan-writing results embed it, so SKILL.md and the quickstart are the routing source. A reference photo with no variation still needs the Variation route today (Quick takes no photo): a product gap, not a routing rule. |
| 61 | One approval per paid batch, one snapshot each (2026-10-10) | Owner feedback: an experiment can contain several paid executions (a calibration, then the rest, then retries), so "approve once" must not read as "the whole experiment is approved". The mechanism already enforced it (`run` reads only an immutable snapshot and requires its fingerprint; a different row set is a different fingerprint); the wording did not. SKILL.md rule 1 now states the principle (each paid batch is approved once and each approval authorizes exactly one execution snapshot) with its consequences: the calibration is approved on its own, the main batch is quoted and approved again, any change to directions, selected rows, model, size or prompt needs a new preflight, an approved snapshot never changes, and the approval message states its scope. Two different yeses are named: a plan confirmation (spends nothing) and a spend approval (a fingerprint); `skills/plan.md` and `skills/direction.md` say "the one confirmation" for the former and are left unchanged (their text is embedded in the advisor's measured results), so SKILL.md says what they mean. New behaviour: `run` prints a note when experiment.xlsx was saved after the approved snapshot (it runs the snapshot, not the edits) and, after a run, lists the ticked rows that were not part of the approval; preflight says what its fingerprint authorizes. |
| 62 | Approval semantics, second review (2026-10-10) | A second outside review of item 61 found three things, checked against the code. (1) **Right:** `run` warned about later edits by comparing file times, which gives false positives (re-saving an unchanged workbook) and false negatives. Now `snapshot_drift` re-runs a non-writing preflight of the current workbook and compares, row by row, model, prompt, size, aspect, quantity, mode, quality and reference hashes with the approved snapshot, and names each row and field that differs, including a row unticked after approval that will still run; rows that already ran and retry snapshots are not compared. The fingerprint covers, per row, id, compiled prompt, model, size, aspect, quantity, mode, quality and reference-file hashes, plus the known total and unverified rows (not file times); the docs now say so. (2) **Half right:** `--max-usd` was never a hard cap, and the guard already counts in-flight samples at the highest real price seen, but a sample with no known price counts as zero until it bills and in-flight requests still bill. The docs no longer call it protection beyond that, and the run result prints the limit and any overshoot. (3) **Already true in code, now stated:** `run --confirm` (and `--again`) is the only spending command; `retry` prepares a new snapshot, `add-takes` adds ticked rows, `recover` only re-downloads images already billed from their recorded URL with no new request; `Unknown` is retried only after the user has checked the usage log, as a new quote. **Not done:** the guard does not report how many samples were in flight when it stopped. |
| 63 | Workload warnings for every plan (2026-10-10) | A third outside review asked why Creative Direction was exempt from plan warnings. Checked: the exemption covered only the dimension-shape warnings (more than 4 variable dimensions, more than 4 values per dimension, more than about 150 valid combinations, which measures the space of combinations, not the images to be made). But no plan, Variation or Creative Direction, was ever warned about its actual workload or cost, and the 3 to 10 directions and 4 images per direction of `skills/direction.md` were not checked by any tool. The dry run now warns, for every plan: a first batch of more than 60 images (rows x takes, spread included), a known cost above $2.00, more than 24 images with no verified price; and for plans with `batch_by`: fewer than 3 or more than 10 directions (two only on request) and any direction that would get fewer than 4 images. Directions stay exempt only from the dimension-shape limits. All are advisory (the quote, the one approval per batch and `--max-usd` still gate any spend); thresholds are constants in `scripts/experiment.py`. **Not done:** takes beyond the first batch (`add-takes`) are not part of the plan-time warning. |
| 64 | Editing boundary and the two-part results report (2026-10-10) | Two more review points, checked. (1) **Where the user may edit:** after the plan confirmation the workbook stays editable; a `preflight` freezes a snapshot and the user's yes approves exactly that one. A change after approval, even dropping one image, never alters the approved snapshot (the command line cannot run part of a snapshot): preflight again, a new fingerprint, a new quote, a new approval. Written into SKILL.md rule 1 and the quickstart. (2) **Results are reported in two parts:** the execution result (facts from the run: generated, failed, Blocked, Unknown, cost) and the visual checks (pass, fail or can't tell per image and check, with a short reason, for text checks what was read letter by letter). The review is stated to be the assistant's own reading, not an independent verification; a subjective requirement gets one observation, not pass or fail; `visual_checks` should be written as things that can be seen on each image. After a run the tool prints the plan's visual checks labelled as unverified. **Not done:** nothing checks automatically whether the assistant actually reviewed every image; the blind vision benchmark (docs/proposal-llm-fit-advisor.md) shows five models read headlines reliably, not that a given run did. quickstart.md is now about 9 KB (it was about 6 KB at the shorter-docs change); a trim pass is open. |
| 65 | End-to-end test of the latest workflow (2026-10-10) | A real run on a fresh copy (3-direction NIGHT MARKET flyer, 13 images, $0.1463, three approvals, real Excel) confirmed the approval semantics, the content-based drift notice, the spending-limit line, the visual-checks list, retry/recover/add-takes, `--again` and the Excel side copy. Six findings, fixed: (1) preflight printed one near-identical line per row that already had its image (11 lines for a 1-image batch, because the file name made each line differ): now one grouped line; (2) wording lives in `out/<name>/plan.json`, not the workbook, and nothing said so or noticed a change: documented, and the ledger now stores a plan hash so preflight says when plan.json changed after the build (consent acknowledgement excluded; old ledgers say nothing); (3) the workbook Read me said "only ticked rows are generated": it now says ticking starts nothing, generation follows an approved quote, saved edits after an approval are in the next quote; (4) the dry run, where the plan is confirmed, now prints the indicative price when no price is verified; (5) "exactly these 1 image(s)" now reads "exactly 1 image"; (6) a thumbnail grid hid a lowercase i in a neon headline (NiGHT) that only showed at full size: the results rules and the printed visual-checks note now say to read text on the full-size image or a crop. quickstart.md was trimmed (about 9.3 KB). **Not covered by the test:** reference photos and person consent, a real Failed or Unknown run, quick-mode generation, Mac and Linux. |
| 66 | Trim pass and a fourth cold start (2026-10-10) | `SKILL.md` went from 17.7 KB to 14.7 KB and `references/quickstart.md` from 9.3 KB to 8.9 KB by writing each topic once: the approval rules live in SKILL.md rule 1 and the quickstart keeps the message template and a short list; the intro table, the repeated route table, the repeated calibration paragraph and duplicate "what not to do" bullets went; no rule was dropped. `skills/plan.md` and `skills/direction.md` are unchanged (the advisor's results embed them). The doc tests now ignore line wraps and pin lower size limits (SKILL.md under 15.5 KB, quickstart under 9.3 KB). Checks: routing 12 of 12 twice on the trimmed text, same as before the trim. A fourth fresh-agent cold start (same brief) reached a valid calibration preflight and wrote the plan confirmation and the spend-approval message, reading about 34 KB of docs (README, SKILL.md, quickstart, one example): more than the third cold start (about 30.5 KB) because the approval and warning rules were added in between, and less than the untrimmed text would have been (about 37.5 KB); 83.2k tokens against 79.9k, with two extra messages to write. The cold start found that the preflight did not say which rows a batch holds, so the approval message could not name them without opening the snapshot: the report now lists the rows of a batch (up to 12) with their values, takes marked. **Not done:** `skills/plan.md` and `skills/direction.md` (17 KB each) are not trimmed; doing so would make the advisor's measured results refer to old text, so it needs a re-run (about $0.7 for the plan-writing evaluation, and the Direction runs) and is a separate decision. |
| 67 | Coverage test of what the end-to-end test missed (2026-10-10) | Run for real, $0.0557 in two approved batches. **Verified live:** a wrong token (every row Failed, "rejected, not charged", $0, "Nothing further was submitted"); `retry` of those rows into a new quote and approval (quick mode, 2 images, $0.0118); a real **Unknown** (the proxy pointed at a closed port, so the submit got no reply: recorded Unknown with submit time and request hash, never retried, `retry` refuses it, `retry --include-unknown` only prepares a quote, the gateway billed nothing); **person photos** (`plan` refuses without consent and creates nothing, `acknowledge-person` only after the user said yes, preflight accepts, `run` refuses when consent is removed, both page builders refuse to publish); a **reference photo** (2 images, $0.0439, identity kept: the red mug, its white label and EMBER, even the reference's own flaws); the quick-mode gallery page. **Five findings, fixed:** (1) the quick-mode gallery's one-file copy was 28 MB for two images (raw PNGs, each used about eight times, over the 16 MB an artifact may weigh): it now embeds recompressed JPEGs (about 1.5 MB) and warns above 16 MB; (2) the page builder warned that a round with no image (every sample failed) was never published; (3) the new "Rows in this batch" list was empty for quick-mode rows: it shows the model; (4) the drift note called every row "unticked or removed" when a fresh preflight had refused them (consent removed): it now quotes the issue; (5) reference rows had no indicative price on a fresh clone: one real measured reference price is now in `references/price-hints.json` (exact key only, other reference sizes get no guess). **Not covered:** Mac and Linux (this machine has Windows and no WSL distribution), a person photo that is a real person, a gateway 5xx on a real image task, edits saved in Excel while a run is in progress. |
| 23 | Column protection for system columns | **Optional**, decided after the MVP workbook is tried |
| 24 | Reading the workbook while Excel has it open | **Partly verified:** Excel open, save, checkbox and edits confirmed on the user's machine; reading under a lock verified with a simulated deny-write lock only (Excel was not running) |
| 25 | LibreOffice rendering of the checkbox column | **To verify** |
