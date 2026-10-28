import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';
import { npmRoot } from './lib.mjs';

// Known high advisories accepted for a limited time. Each entry is scoped to one installed path,
// states why it cannot be reached by this tooling, and expires so CI fails again for a fresh review.
export const acceptedAdvisories = [
  ...['GHSA-xjj6-f33q-929c', 'GHSA-2m5v-pch6-74hx', 'GHSA-qrxm-fqhp-jwvm'].map((id) => ({
    id,
    node: 'node_modules/npm/node_modules/undici',
    expires: '2026-11-27',
    reason: 'Bundled in the pinned npm CLI and reached only through node-gyp; release tooling runs pack, '
      + 'publish, view and dist-tag, which fetch through make-fetch-happen. No npm release bundles a fixed '
      + 'undici yet (11.21.1 and 12.2.1 ship 6.29.0).',
  })),
];

const blocking = new Set(['high', 'critical']);

// findBlockingAdvisories returns high or critical advisories not covered by a current acceptance.
// Entries whose `via` only names other packages are transitive and reported through those packages.
export function findBlockingAdvisories(report, today, accepted = acceptedAdvisories) {
  const findings = [];
  for (const [name, vulnerability] of Object.entries(report.vulnerabilities ?? {})) {
    for (const advisory of vulnerability.via ?? []) {
      if (typeof advisory !== 'object' || !blocking.has(advisory.severity)) continue;
      const id = String(advisory.url ?? '').split('/').pop();
      const nodes = vulnerability.nodes ?? [];
      const covered = nodes.length > 0 && nodes.every((node) => accepted.some(
        (entry) => entry.id === id && entry.node === node && today <= entry.expires));
      if (!covered) findings.push({ name, id, severity: advisory.severity, nodes, title: advisory.title });
    }
  }
  return findings;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = spawnSync('npm', ['audit', '--json', '--registry', 'https://registry.npmjs.org'],
    { cwd: npmRoot, encoding: 'utf8', timeout: 120000, shell: process.platform === 'win32' });
  let report;
  try {
    report = JSON.parse(result.stdout);
  } catch {
    console.error(`npm audit did not return JSON: ${result.error?.message ?? result.stderr}`);
    process.exit(1);
  }
  if (report.error) {
    console.error(`npm audit failed: ${report.error.summary ?? JSON.stringify(report.error)}`);
    process.exit(1);
  }
  const findings = findBlockingAdvisories(report, new Date().toISOString().slice(0, 10));
  for (const entry of acceptedAdvisories) {
    console.log(`accepted until ${entry.expires}: ${entry.id} at ${entry.node}`);
  }
  if (findings.length > 0) {
    for (const finding of findings) {
      console.error(`${finding.severity} ${finding.id} ${finding.name} (${finding.nodes.join(', ')}): ${finding.title}`);
    }
    process.exit(1);
  }
  console.log('npm audit: no unaccepted high or critical advisories');
}
