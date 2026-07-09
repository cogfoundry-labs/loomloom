# Examples and Patterns

---

## Pattern 1: Single-Step Generation

**Use for:** single-step copy generation, rewriting, translation, or classification.

```json
{
  "meta": {
    "name": "Single Text Generate",
    "description": "Single-step text generation"
  },
  "steps": [
    {
      "stepId": "stp_text01",
      "displayName": "Generate",
      "executionUnit": "text-generate",
      "defaultModelRef": { "modelKey": "google/gemini-2.5-flash" }
    }
  ],
  "inputSchema": {
    "fields": [
      {
        "key": "prompt",
        "label": "Prompt",
        "required": true,
        "valueType": "string",
        "order": 1
      }
    ],
    "sampleRows": [
      {
        "values": {
          "prompt": "Write a short launch announcement"
        }
      }
    ]
  },
  "fieldBindings": [
    {
      "fieldKey": "prompt",
      "stepId": "stp_text01",
      "paramKey": "prompt",
      "bindMode": "shared"
    }
  ]
}
```

---

## Pattern 2: Linear Chain

**Use for:** two serial steps where the second step consumes the first step's output, such as draft -> polish.

```json
{
  "meta": {
    "name": "Draft and Polish",
    "description": "Draft text, then polish the draft"
  },
  "steps": [
    {
      "stepId": "stp_draft1",
      "displayName": "Draft",
      "executionUnit": "text-generate",
      "defaultModelRef": { "modelKey": "google/gemini-2.5-flash" },
      "instruction": "Write a draft based on the prompt."
    },
    {
      "stepId": "stp_polish",
      "displayName": "Polish",
      "executionUnit": "text-generate",
      "defaultModelRef": { "modelKey": "google/gemini-2.5-flash" },
      "dependsOn": ["stp_draft1"],
      "upstreamBindings": [
        {
          "inputPort": "prompt",
          "sourceStepId": "stp_draft1",
          "sourcePort": "output"
        }
      ],
      "instruction": "Polish and improve the draft above."
    }
  ],
  "inputSchema": {
    "fields": [
      {
        "key": "topic",
        "label": "Topic",
        "required": true,
        "valueType": "string",
        "order": 1
      }
    ]
  },
  "fieldBindings": [
    {
      "fieldKey": "topic",
      "stepId": "stp_draft1",
      "paramKey": "prompt",
      "bindMode": "shared"
    }
  ]
}
```

**Key points:**

- `stp_polish.prompt` comes from UpstreamBinding. There is no FieldBinding that writes to `prompt`, preserving binding target uniqueness.
- `instruction` is a system-level instruction written by the template author. It is hidden from users and cannot be edited by users.

---

## Pattern 3: Step-Level Fan-In Review Summary

**Use for:** reviewing the same material from multiple perspectives in parallel, then summarizing the results. A typical case is multi-role PRD review.

**Structure:**

```text
Multiple parallel steps (product, engineering, and risk review)
  ->
One summary step (upstreamBindings receive multiple upstream outputs)
```

```json
{
  "meta": {
    "name": "PRD Review and Summary",
    "description": "Review one PRD from product, engineering, and risk perspectives, then summarize"
  },
  "steps": [
    {
      "stepId": "stp_prod01",
      "displayName": "Product Review",
      "executionUnit": "text-generate",
      "instruction": "Review the PRD from a product perspective. Focus on goal clarity, scope boundaries, requirement consistency, user value, and ambiguous behaviors.",
      "defaultModelRef": { "modelKey": "google/gemini-2.5-flash-lite" }
    },
    {
      "stepId": "stp_eng001",
      "displayName": "Engineering Review",
      "executionUnit": "text-generate",
      "instruction": "Review the PRD from an engineering perspective. Focus on implementation complexity, technical risks, edge cases, dependencies, and missing constraints.",
      "defaultModelRef": { "modelKey": "google/gemini-2.5-flash-lite" }
    },
    {
      "stepId": "stp_risk01",
      "displayName": "Risk Review",
      "executionUnit": "text-generate",
      "instruction": "Review the PRD from a risk and rollout perspective. Focus on launch risk, support burden, data or compliance concerns, and operational readiness.",
      "defaultModelRef": { "modelKey": "google/gemini-2.5-flash-lite" }
    },
    {
      "stepId": "stp_summary",
      "displayName": "Summary",
      "executionUnit": "text-generate",
      "defaultModelRef": { "modelKey": "google/gemini-2.5-flash-lite" },
      "dependsOn": ["stp_prod01", "stp_eng001", "stp_risk01"],
      "triggerPolicy": "require_all",
      "upstreamBindings": [
        {
          "inputPort": "prompt",
          "sourceStepId": "stp_prod01",
          "sourcePort": "output"
        },
        {
          "inputPort": "prompt",
          "sourceStepId": "stp_eng001",
          "sourcePort": "output"
        },
        {
          "inputPort": "prompt",
          "sourceStepId": "stp_risk01",
          "sourcePort": "output"
        }
      ],
      "instruction": "Summarize the review results above into one final PRD review. Consolidate overlapping findings, call out the most important risks, and end with a concise conclusion."
    }
  ],
  "inputSchema": {
    "fields": [
      {
        "key": "prd_content",
        "label": "PRD Content",
        "required": true,
        "valueType": "string",
        "order": 1
      }
    ],
    "instructions": [
      "Each row represents one PRD review task.",
      "Fill PRD Content with the PRD text. The same field is bound to each review step.",
      "The summary step depends on all review steps and receives all review outputs."
    ]
  },
  "fieldBindings": [
    {
      "fieldKey": "prd_content",
      "stepId": "stp_prod01",
      "paramKey": "prompt",
      "bindMode": "shared"
    },
    {
      "fieldKey": "prd_content",
      "stepId": "stp_eng001",
      "paramKey": "prompt",
      "bindMode": "shared"
    },
    {
      "fieldKey": "prd_content",
      "stepId": "stp_risk01",
      "paramKey": "prompt",
      "bindMode": "shared"
    }
  ]
}
```

**Key points:**

- `prd_content` is bound separately into the three review steps. The three steps receiving the same field value does not mean there is a new input fan-out syntax.
- If the three steps should use different material, define three fields such as `prd_product`, `prd_engineering`, and `prd_risk`, and bind them to the corresponding root steps.
- `stp_summary` is the true step-level fan-in step. It depends on three upstream steps and binds all three `output` ports to its own `prompt`.
- `text-generate.prompt` allows multiple text sources, so this fan-in is valid.
- `triggerPolicy` controls the summary node's trigger behavior. Here it explicitly uses `require_all`, meaning all three perspectives must succeed before summary runs. Omitting the field also defaults to `require_all`.

If the business workflow allows the summary to continue when some perspectives fail, change the summary step to:

```json
{
  "stepId": "stp_summary",
  "displayName": "Summary",
  "executionUnit": "text-generate",
  "defaultModelRef": {
    "modelKey": "deepseek/deepseek-v4-flash"
  },
  "dependsOn": ["stp_prod01", "stp_eng001", "stp_risk01"],
  "triggerPolicy": "allow_partial",
  "upstreamBindings": [
    {
      "inputPort": "prompt",
      "sourceStepId": "stp_prod01",
      "sourcePort": "output"
    },
    {
      "inputPort": "prompt",
      "sourceStepId": "stp_eng001",
      "sourcePort": "output"
    },
    {
      "inputPort": "prompt",
      "sourceStepId": "stp_risk01",
      "sourcePort": "output"
    }
  ],
  "instruction": "Create a consolidated review from available upstream perspectives."
}
```

`allow_partial` waits until all upstream steps reach terminal states, then runs the summary if at least one upstream step succeeded. If all upstream steps fail, the summary does not run. Failed upstream error reasons are not passed as business text into the summary step.
