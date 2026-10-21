# Local draft preview implementation

Supersedes the unreleased remote preview plan. See [workflow viewing](../guides/workflow-preview.md). Shared compiled UI is embedded in CLI; loopback service owns local snapshots and file watching. No remote preview lifecycle, writer secret, lease or Redis. Saved-version links stay online and retain ownership checks.
