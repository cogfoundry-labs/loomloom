# Limits

Public v1 requires at least one step and field, explicit bindings for multi-upstream fan-in, one binding source per parameter target, and at most one multi-value source per ParamBinding. Repeated input-port bindings require `allowMultiple`. Provider/mode cannot be overridden. Model, account, file, and provider limits are dynamic facts.

Additional stable limits:

- Step IDs use the fixed v1 pattern and the dependency graph is acyclic.
- Video prompt/image ports do not accept repeated bindings.
- Workbooks are version-bound and should be regenerated after schema changes.
- Non-user source kinds require defaults.

Dynamic values such as available models, provider parameter ranges, account balance, upload size, and retention must be queried from the target environment. Multi-upstream is enabled in the standard server/User/Official service wiring; a custom service assembly can still disable it.
