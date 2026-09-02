#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import {
  assertSafeGeneratedPath,
  betaVersion,
  npmCommand,
  npmCommandArgs,
  parseArgs,
  repoRoot,
} from "./lib.mjs";

const args = parseArgs(process.argv.slice(2), {
  tarballs_dir: path.join(repoRoot, "dist", "npm-tarballs"),
  registry: "https://registry.npmjs.org",
  publish: false,
  timeout_ms: "30000",
  publish_timeout_ms: "300000",
});
const tarballsDir = assertSafeGeneratedPath(args.tarballs_dir);
const timeout = Number.parseInt(args.timeout_ms, 10);
const publishTimeout = Number.parseInt(args.publish_timeout_ms, 10);
if (
  !Number.isSafeInteger(timeout) ||
  timeout <= 0 ||
  !Number.isSafeInteger(publishTimeout) ||
  publishTimeout <= 0
) {
  throw new Error("--timeout-ms and --publish-timeout-ms must be positive integers");
}
const manifest = JSON.parse(
  fs.readFileSync(path.join(tarballsDir, "publish-manifest.json"), "utf8"),
);

if (betaVersion(manifest.releaseTag) !== manifest.version) {
  throw new Error("publish manifest release tag and npm version do not match");
}
if (manifest.packages.length !== 7 || manifest.packages.at(-1)?.role !== "main") {
  throw new Error("publish manifest must contain six platform packages followed by the main package");
}
for (const [index, record] of manifest.packages.entries()) {
  const expectedRole = index === manifest.packages.length - 1 ? "main" : "platform";
  const expectedTag = expectedRole === "main" ? "beta" : "npm-bootstrap";
  if (
    record.role !== expectedRole ||
    record.publishTag !== expectedTag ||
    path.basename(record.tarball) !== record.tarball
  ) {
    throw new Error(`invalid publication order or tarball path for ${record.name}`);
  }
}
if (args.publish && args.confirm_tag !== manifest.releaseTag) {
  throw new Error(
    `--publish requires --confirm-tag ${manifest.releaseTag}; no registry writes were attempted`,
  );
}

function npm(commandArgs, options = {}) {
  return spawnSync(npmCommand, [...npmCommandArgs, ...commandArgs, "--registry", args.registry], {
    encoding: "utf8",
    stdio: options.capture ? ["ignore", "pipe", "pipe"] : "inherit",
    timeout: options.timeout ?? timeout,
  });
}

function registryMetadata(record) {
  const result = npm(
    ["view", `${record.name}@${record.version}`, "dist", "--json"],
    { capture: true },
  );
  if (result.error) {
    throw new Error(`failed to inspect ${record.name}@${record.version}: ${result.error.message}`);
  }
  if (result.status === 0) {
    return JSON.parse(result.stdout);
  }
  if (/E404|404 Not Found/.test(`${result.stderr}\n${result.stdout}`)) {
    return null;
  }
  throw new Error(`failed to inspect ${record.name}@${record.version}: ${result.stderr.trim()}`);
}

for (const record of manifest.packages) {
  const existing = registryMetadata(record);
  if (existing) {
    if (existing.integrity !== record.integrity || existing.shasum !== record.shasum) {
      throw new Error(
        `${record.name}@${record.version} already exists with different immutable tarball integrity; ` +
        `expected integrity=${record.integrity} shasum=${record.shasum}, ` +
        `got integrity=${existing.integrity} shasum=${existing.shasum}`,
      );
    }
    console.log(`verified existing ${record.name}@${record.version}`);
    continue;
  }

  if (!args.publish) {
    console.log(`would publish ${record.name}@${record.version} with tag ${record.publishTag}`);
    continue;
  }

  const tarball = path.join(tarballsDir, record.tarball);
  const published = npm(
    [
      "publish",
      tarball,
      "--access",
      "public",
      "--tag",
      record.publishTag,
    ],
    { timeout: publishTimeout },
  );
  if (published.status !== 0) {
    throw new Error(`failed to publish ${record.name}@${record.version}`);
  }
  const verified = registryMetadata(record);
  if (!verified || verified.integrity !== record.integrity || verified.shasum !== record.shasum) {
    throw new Error(`registry read-back failed for ${record.name}@${record.version}`);
  }
  console.log(`published and verified ${record.name}@${record.version}`);
}

console.log(args.publish ? "npm publication completed" : "npm publication plan completed without writes");
