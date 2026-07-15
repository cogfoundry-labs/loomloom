# TemplateSpec quickstart

This guide creates a local draft from a complete example and validates it with `check`. It does not create a remote template or incur model cost.

## Prerequisites

- Install the current LoomLoom CLI.
- Confirm that `loomloom template-spec docs` and `check` run.
- Prepare an empty directory for `template.json`.

## 1. Read the reference shipped in the CLI

Read the syntax entry point first, then consult the input, step, binding, and execution-unit references as needed. Access to a source checkout is not required:

```bash
loomloom template-spec docs spec
loomloom template-spec docs inputs
loomloom template-spec docs steps
loomloom template-spec docs bindings
loomloom template-spec docs execution-units
loomloom template-spec docs examples
```

The JSON files listed by `examples` are canonical examples shipped with the CLI. A fixed number of parallel DAG branches uses independent root steps and paired `dependsOn` / `upstreamBindings`; do not add `branch` or `parallel` properties, and do not use `expanded` in place of fixed branches.

## 2. Copy the minimal template

Copy `examples/valid/single-text-generation.json`. It contains one `string` input field named `content`, one `text-generate` step, and a FieldBinding from `content` to the step prompt.

## 3. Select a model

```bash
loomloom template-spec models text-generate
```

Replace the example `defaultModelRef.modelKey` with a model ID returned by the target environment. Model catalogs are dynamic, so an example ID is not guaranteed to work everywhere.

## 4. Change the business definition

Initially change only:

- `meta.name` and `meta.description`.
- `steps[0].displayName` and `instruction`.
- Input-field label, description, and presentation.

Do not initially change a step ID, field key, valueType, or binding; these values reference one another.

## 5. Check locally

```bash
loomloom template-spec check ./template.json
```

A `valid` result confirms locally known structure and authoring rules. It does not validate every dynamic capability of the target environment.

## 6. Understand the result

At run time, each input row supplies `content`; the FieldBinding writes it to the step prompt; the author instruction remains fixed; and the step emits a `text/plain` artifact.

Remote create/create-version is a separate write action and requires confirmation before execution.
