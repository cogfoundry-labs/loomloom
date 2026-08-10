# Templates and versions

A template has a long-lived identity; each version is an immutable definition snapshot. Add a version when inputs, steps, bindings, instructions, models, or visible outputs change. Workbooks belong to a version and should be downloaded again. Market Listing Versions are separate publish snapshots and do not update automatically.

## Why versions are immutable

Runs, workbooks, and Market listings must remain traceable to the exact definition used at execution time. Editing an old version in place would make one version ID mean different fields, steps, or costs at different times.

## Changes that require a version

- Add, remove, rename, or retype an input field.
- Change required, multi-value, default, or presentation behavior.
- Add or remove steps, dependencies, or bindings.
- Change fixed instructions, models, static parameters, or visible outputs.

Local drafts can be edited freely before remote creation. After creating a version, use `create-version` and download a new workbook.
