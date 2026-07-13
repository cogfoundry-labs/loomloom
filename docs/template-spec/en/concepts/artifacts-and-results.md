# Artifacts and results

Step executions produce typed artifacts with producer identity and opaque storage references. Never parse artifact URIs. Use result rows for structured data, result workbooks for server-aligned input/output, and artifact list/download for individual files.

Artifacts carry MIME type, port, producer step/run, execution index, and deterministic ordinal. Their URI is an opaque platform reference; clients must not assume OSS, S3, GS, or data-URI syntax.

## Result views

- Result rows align structured outputs with source rows.
- Result workbook is assembled server-side from the submitted input snapshot and artifacts.
- Artifact list/download accesses individual text, image, video, or file outputs.

Intermediate and terminal steps can both produce artifacts. Verify step status, MIME, and business acceptance criteria rather than treating any artifact as proof of complete delivery.
