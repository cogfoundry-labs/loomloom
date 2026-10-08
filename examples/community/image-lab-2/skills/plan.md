---
name: plan
description: Image Lab 2 experiment mode, stages 2-3 (understand and schema). Turns a brief, and optionally reference photos, into a confirmed plan.json: the intent, what is fixed, 2-4 dimensions with a few values each and the wording for them, rules for combinations that make no sense, and one plan confirmation from the user. Judgment only; the matrix, the workbook and every price are computed by scripts.
---

# plan

You decide what needs judgment. Scripts do the rest. Your output is one file,
`plan.json`, plus one confirmation from the user. Nothing here spends money.

Read `../references/plan-schema.md` for the file's shape (and the intent list in
`../references/generation-policy.md`; you only need its intent names). Run commands from the
skill folder, `PYTHONUTF8=1 python scripts/image.py ...` on Windows.

## Do you need this stage at all?

Use **quick mode** (SKILL.md) and write **no plan** when ALL of these hold: the user gave one
finished prompt, there is no reference photo, no controlled variation is asked for, and at
most 8 images are wanted. "Finished prompt" means text the user presents as the thing to
generate ("generate that: a red apple on a white table, studio photo"); it is used verbatim,
so do not expand or improve it. A short brief that asks you to explore options is not a
finished prompt: use this stage. When in doubt between the two, the user's words decide:
"explore", "vary", "different X" or a photo means this stage.

Use this stage when they supply a reference photo, ask to vary things, or want more than 8
images. Say which route you chose in one line ("one prompt, no photo, 4 images: quick
mode").

**Creative Direction.** If the user wants directions explored, or the brief is creatively open-ended and they chose to explore it, run
`skills/direction.md` first. It produces a Direction Sheet (3 to 10 directions, the user's idea as Direction 1) and the plan below is then written from it.
For a direction plan the usual planner limits do not apply (more than four variable dimensions and more than 150 valid combinations are normal there, with
`batch_by: "direction"`); the validity limits enforced by `check` still do.

**If the route is quick mode, this stage ends here:** write no `plan.json`, ask for no plan
confirmation (the quote in SKILL.md quick mode step 3 is the confirmation the user sees
before any spend), and continue with SKILL.md's quick steps. Take the verbatim prompt from
the user's words (the text after "Generate that" or the quoted/pasted prompt), and map the
count with SKILL.md's table (1, 2, 4 or 8; a request for 6 becomes 8, for 5 becomes 4; say
so). `python scripts/image.py check --plan FILE` on a file containing `{"route": "quick"}`
prints `ROUTE quick`, which is the marker an evaluation harness can use.

## Steps

### 1. Capture

Take the brief. Put reference files in a `refs/` folder next to where you will write
`plan.json`; never modify the originals. Ask a question only if you cannot proceed
without the answer: what is being made, for what use. One or two questions at most.

### 2. Understand

- **Intent.** One name from the intent list. It drives model choice and default sizes only,
  so a wrong one costs quality, not structure, and the user can correct it. Choose by what
  the image is *for* ("poster", "article hero", "profile picture", "product page"); a
  product photo for a shop is `product / e-commerce shot` even if the request is about
  lighting; use `generic` only when nothing fits. The list has no still-life or interior
  category: pick the use, not the subject. When no use is stated and the image is a single
  object on a plain surface, `product / e-commerce shot` is the tie-break; for a scene with
  no stated use, `generic`. The user can always correct it.
- **Reference roles.** Give each reference one role: `product` (keep it recognizable),
  `person` (keep the likeness), `style` (borrow palette and mood only), `general`. A
  reference with several items (a bottle and its dispenser) is still one `product`
  reference: name every item in `fixed`.
  If a reference shows a **person**, set `contains_person: true`, leave
  `consent_acknowledged` null, and include the person notice (SKILL.md, experiment mode
  step 1) in the plan confirmation below. This applies to a user's own selfie too: the
  notice says the photo is sent to the model provider, which they should know. Only after
  the user says yes to **the notice itself** in chat, run
  `acknowledge-person --plan plan.json`; `plan --out` is refused until then. A reply that
  only edits the plan is not a yes to the notice: ask again. Never acknowledge on the
  user's behalf.
- **Fixed vs variable.** Fixed means it must not change in any image. It always includes
  the identity of a product or person reference. It also includes anything the brief
  requires, and **anything the brief leaves unspecified that would otherwise drift and
  confound the comparison**: the background or surface when the brief varies lighting or
  camera, "no people" or "no text in the image" when they were not asked for. Write each
  Fixed item as one plain phrase; it becomes part of every prompt (the compiled prompt
  appends your Fixed items to the reference sentence, and builds the text prompt from
  them). Say in the confirmation which Fixed items you added beyond what the brief said, so
  the user can drop them. Everything the user wants to explore is variable.
  **Write Fixed text for the model, not for yourself:** the model only sees "the reference
  image", never an id, so write "the bottle in the reference image", never "the bottle in
  ref1" (the dry run warns when a Fixed item names a reference id). When the subject is the
  scene itself (a coffee shop interior), put the subject in `fixed` and vary the *kind* of
  scene in an `environment` dimension. For a person reference, the default sentence tells the
  model not to copy the photo's pose, framing, clothing or background; if you want clothing
  or framing to stay constant across the set, say what you want in `fixed`.

### 3. Schema and controls

Pick only the dimensions that matter for this brief, from: environment, composition,
camera, lighting, style, mood, pose / action, aspect (only if the use needs a particular
shape: banner, portrait, square). **Two to four variable dimensions.** A single-value `aspect` (one fixed shape such as 4:5) is not a variable and does not count; an `aspect` with several values counts as one.

**If the user asks for more than four**, keep the four that most change what a viewer sees
(usually environment or subject variation, lighting, camera, style) and merge or defer the
rest: mood is mostly expressed through lighting and style, composition through camera. Say
so, in the confirmation table as rows marked "Not in this round" ("Mood: not in this round
(covered by lighting and style); offered as round 2").

For each variable dimension propose **three values, at most four**. Two are fine when the
choice is binary, or when only two values are safe (with a reference photo the tested camera
vocabulary has one reliable value, `eye-level front`, and `three-quarter high`, which is weak: the
model copies the photo's pose, so the effect is subtle. Do not pad with an untested value just to
reach three; if you offer `three-quarter high` or an untested value, say how weak or untested it is). Prefer values a person can tell apart at a glance; avoid
near-duplicates ("soft light" and "gentle light").

Look at the starter vocabulary first:

```
python scripts/image.py controls --dimension lighting
python scripts/image.py controls
```

- A value in the catalog needs no wording from you; write it as a plain string.
- A value marked `UNTESTED` still works, but tell the user its wording is starter
  wording and the first batch will show whether it reads as intended.
- **Check the scope tag.** Wording tagged "written for a product on a surface" (dramatic
  spotlight at counter height, low angle on a surface, flat lay, photorealistic
  commercial photograph) must not be used for a person or a whole-room scene: write your
  own fragment. Portrait entries exist for camera and style.
- A value not in the catalog needs wording from you: give `{value, fragment}`. One short
  natural phrase that states the **visible result** ("Deep orange sunlight from low at the
  side, a strong warm color cast"), not an instruction to the model about itself.
- **Wording must be unmistakable, or it is invisible.** Blind testing showed mild wording
  (for example "warm golden-hour light") produced no visible difference, and a strong
  concrete cue ("a strong orange color cast over the whole scene") was identified 9 of 9
  times. State the cue the viewer will see.
- **With a reference photo, camera wording needs care.** The model copies the pose of the
  reference: strong camera wording makes it tilt the product. Give camera values a separate
  `fragment_with_reference` that is milder, and mark any value that still does not work
  there with `"reliability": {"reference": "unreliable"}` (preflight then warns). `low
  angle` is unreliable with a reference and `three-quarter high` is weak (it barely changed the
  angle in 2 of 3 blind tests); prefer `eye-level front`, and tell a user who asked for "different camera
  angles" with a photo that angle changes are limited by the photo's pose.
- **A style dimension and the default sentence.** When the plan has a `style` dimension the
  default closing sentence is the neutral "High quality." (without it, "Realistic, high
  quality."). If any value is not photographic, or a default does not suit, set
  `prompt.text_suffix` / `prompt.reference_suffix` yourself.
- **Do not let a style endanger identity.** For a person reference, illustration, painting
  and 3D styles change the face; offer photographic treatments (portrait styles, backgrounds,
  lighting) and say so.
- Do not put identity wording in a dimension; identity belongs in `fixed`.
- `aspect` values are ratios (`1:1`, `4:5`, `16:9`). Not every model can honor an exact
  ratio; preflight shows what each row really gets, and the dry run prints "Size on <model>"
  for each ratio (and says when the ratio is not exact). Exact ratios: the Gemini models (a real
  ratio field) and the models that take any WxH on a 16-pixel grid (GPT Image 2.5 Sunburst and
  Flare); the others get the nearest size they list. If an exact ratio matters (a banner, an
  Instagram 4:5), check the "Size on" line and, if it says "not exact", offer
  `model_strategy: "fixed:<a model that is exact>"`. Do not write wording for `aspect`, and omit it
  when the intent's default size (also printed by the dry run) already suits the use.

**Visual checks.** For every requirement that models follow only loosely and that no
script can verify, add a sentence to `visual_checks` (at most six): "no text, letters or
logos anywhere in the image", "a calm empty area on the left for a headline", "the label and
shape of the product match the reference image", "the face is recognizably the same person".
After the first batch the agent looks at every image and reports which fail. Do not skip this
for banners (headline space), no-text requirements, or reference photos (identity).

**Quality and cost.** With models that have a `quality` setting (GPT Image 2.5 Sunburst and
Flare, gpt-image-2), `auto` lets the model pick an effort level per image, and one request shape
then billed $0.030 for some images and $0.055 for others. For a predictable cost set
`quality` in the plan. The preflight cannot price an explicit quality until one image has been
billed, so say so in the confirmation, give a spend limit you are willing to stand behind, and
for a large batch suggest a 2-image calibration first. Never quote a flat catalog price for a
batch with an explicit size, a long prompt or an explicit quality.

**Text the model must not invent.** Image models add slogans and taglines when a brief has a
typography section. If the copy is fixed, end the plan's `prompt.text_suffix` with an explicit
list of the only allowed text and "no other words, slogans or taglines", and put a matching item
in `visual_checks`.

**Hierarchy dimensions.** When one dimension is a switch that makes other dimensions mean different things (three campaign territories, each excluding different values of color, typography and composition), set `batch_by` to that dimension. The first batch is then split evenly across its values, each with its own covering design; without it a plain pairwise design can be badly unbalanced across the switch (8 / 16 / 7 rows in a measured plan). The dry run's `Why` line lists the share per value and any value whose share is below its covering minimum.

**Takes.** A row is exactly one image. Leave `takes` out (1) for a first exploration; extra images of the same values are added later to the rows that look promising (`add-takes`), not planned up front.

**Names and paths.** A dimension may not be called `id`, `selected`, `model`, `qty`, `take`, `reference`,
`prompt`, `status`, `images`, `image`, `file`, `cost` or `notes` (use `model look`, `outfit`, ...). Reference files are
relative paths inside the plan's folder (`refs/photo.png`), never absolute or with `..`. The intent must
be exactly one of the intent names.

**Constraints.** Add `{"exclude": {dim: value, ...}}` only for combinations that make no
sense or would be unusable (for example a "night" environment with "bright daylight").
Hard rules only; do not encode taste. Every excluded pair must name values that exist.

**Model strategy.** `single` by default. Use `spread` only if the user wants to compare
models; `fixed:<id>` only if they named one.

### 4. Write and check

Write `plan.json`, then check it. Both commands write nothing and spend nothing:

```
python scripts/image.py check --plan plan.json
python scripts/image.py plan --plan plan.json --out ./out/<name> --dry-run
```

`check` prints `PLAN OK: ...` with the dimensions and number of valid combinations, or
`PLAN INVALID` with the reasons (exit 1). The dry run prints the valid combinations, the
recommended first batch (about 30 rows, every pair of values at least once), the cost
estimate, the **price basis** (per image, text or reference call, and how many rows have
no verified price), the **size** each aspect gets on the estimate model, the **wording
used for every value** in this mode, **one sample compiled prompt**, and `Check:` lines for
values with no wording, untested wording (starter wording and wording you wrote yourself),
unreliable values, a Fixed item that names a reference id, a reference file not yet next to
the plan,
and a plan that is wide (over about 150 valid combinations: drop a dimension or a value; a
plan over a million combinations is refused). **Read the sample prompt**: it is exactly the
text a model will see, so check that your Fixed items, wording and closing sentence read
well together and nothing contradicts anything else. Fix what the output flags before
showing the user.

### 5. One plan confirmation

Show the user, in plain words and in one message:

| Dimension | Status | Values |
|---|---|---|
| Product identity | Fixed | the bottle in the reference image, unchanged |
| Camera | Variable | eye-level front, three-quarter high |
| Lighting | Variable | soft daylight, golden hour, dramatic spotlight |
| Mood | Not in this round | covered by lighting and style |

then the rules you added, the intent, the reference roles, any person notice, the visual checks, the Fixed
items you added beyond the brief, which wording is untested, and the dry-run numbers (valid
combinations, first batch size, estimated cost with its price basis and the number of rows
without a verified price). Ask for **one** confirmation or edits, in one turn (a plain
message, or `AskUserQuestion` with Confirm / Edit). Do not ask dimension by dimension.
After edits, rewrite the file and re-run the two checks. Ask again only after a material
change: adding or removing a dimension or value, changing a Fixed item, or adding an
exclude rule. Reworded wording or a reordered list is not material.

If the first batch is larger than the user asked for ("a few looks" and a 27-row batch), say
what the smallest set that still shows every pair of values is (the dry run's "needed"
number) and that they can untick rows in the workbook before anything is generated. Say
plainly that image models follow "no text" and "leave space here" wording only loosely, so
the first batch is how they find out.

Then continue with `plan --out` (writes the workbook) as in SKILL.md. `plan --out` copies
the reference files next to the experiment; they must exist next to `plan.json` by then.

## What not to do

- Do not exceed four dimensions or four values per dimension without telling the user why.
- Do not write wording for a value the catalog already has unless you have a reason; say
  what the reason is.
- Do not offer values you expect not to work (a strong low angle on a reference photo), or
  product-scoped wording for a person or a room.
- Do not infer or invent a person's consent; do not run `acknowledge-person` yourself.
- Do not put a price in the plan or promise a total; the preflight prices rows.
- Do not edit `experiment.xlsx`.
