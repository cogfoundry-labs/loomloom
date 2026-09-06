# Image Lab · showcase (v0.4) — the loomloom workflow

This sub-example is where Image Lab **is** a loomloom workflow, not a set of
direct API calls.

## Why this one needs the compiler

v0.1–v0.3 are N independent gateway tasks — spread across a couple of models, but
still independent (plus, in v0.3, one LLM call after them). The gateway API can
fan out N calls, but it **cannot express a step dependency** — there is no way
to say "generate only after the prompt is tidied" or "judge only after all the
images exist" as one metered job.

That is exactly what a loomloom TemplateSpec is for:

```
stp_tidy   (text-generate)    raw prompt -> a clean, model-ready prompt
   |
stp_gen_a  (image-generate)  \
stp_gen_b  (image-generate)   |  4 parallel branches, all depend on stp_tidy
stp_gen_c  (image-generate)   |
stp_gen_d  (image-generate)  /
   |
stp_judge  (text-generate)    the 4 images + the brief -> ranked pick + why
```

One `loomloom template-spec run` executes the whole graph with maximum safe
parallelism and gives back:

- **one run id** and **one settled charge** (not N separate task costs)
- **per-step status** and artifacts from one `run get`
- a **reusable, versioned spec** that can be packaged as a SkillBot

(Four identical branches here — the showcase demonstrates *dependencies*, not
multi-model. When loomloom wraps more image models as contracts, this mirrors
v0.1's allocation: two model contracts, two branches each.)

## Status

`variants-4.spec.json` is the seed — the four `image-generate` branches, already
`loomloom template-spec check` → `valid` against the live server. The `stp_tidy`
and `stp_judge` `text-generate` steps and the wiring are the v0.4 build.

`stp_gen_*` binds the `google/gemini-2.5-flash-image` fixed-model contract — the
one image model loomloom's TemplateSpec catalog exposes today (the gateway's
other 6 are not yet wrapped as contracts). So this costs loomloom's
image-generation rate ($0.04–$0.30/image), not the gateway rate — which is the
point: you pay for orchestration when the work needs it.

`variants-4.spec.json`'s `subjectRevisionId` points at a **shared** model
contract in loomloom's catalog, not account data — but re-run
`loomloom template-spec check` at v0.4 build time in case the catalog has moved.
