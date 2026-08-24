# Compatibility boundary

<a id="ref-compatibility-write-version"></a>

## TS-VERSION-002: new writes are v2 only

New TemplateVersion writes accept only `template-spec/v2`. Existing v1 versions remain readable and executable from frozen snapshots. Migration reads v1 and creates a new v2 version; it never mutates history in place.

Rehearse on test, run the complete migration on pre-production, then migrate production in batches. Each environment creates its own v2 records through formal services; generated database rows are not copied across environments.
