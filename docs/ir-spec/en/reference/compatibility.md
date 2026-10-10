# Compatibility boundary

<a id="ref-compatibility-write-version"></a>

## TS-VERSION-002: new writes are v2 only

New TemplateVersion writes accept only `template-spec/v2`. Existing v1 versions remain readable and executable from frozen snapshots. Migration reads v1 and creates a new v2 version; it never mutates history in place.

Rehearse on test, run the complete migration on pre-production, then migrate production in batches. Each environment creates its own v2 records through formal services; generated database rows are not copied across environments.

A future bounded major-version compatibility window requires separate cross-team review and does not expand this v2 implementation.

## Four independent versions

| Version | Meaning | Current rule |
| --- | --- | --- |
| `specVersion` | Author JSON format and semantics, such as `template-spec/v2` | One format per template; v1 and v2 are not combined. Code, when, and Loop extend v2 |
| TemplateVersion / `versionId` | An immutable save | Code, binding, or Loop changes create a new version; old Runs retain their original version |
| Local execution contract / `code.contractVersion` / Profile revision | Node input, environment, or frozen execution semantics | Version the layer that changed; Spec and API need not change together |
| API `/loom/v1` | External transport and resource API | API v1 does not restrict templates to Spec v1 |

A new node does not automatically require Spec v3. Review a new major version only for changes to existing semantics that cannot be isolated by a new field, type, or local contract, together with migration of existing data. Unsupported readers must reject the new capability, not ignore Code/Loop and execute an old model node instead.
