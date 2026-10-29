#!/usr/bin/env node

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { runConcurrent } from "./run-concurrent.mjs";
import {
  assertSafeGeneratedPath,
  releaseVersion,
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
  readback_attempts: "30",
  readback_interval_ms: "10000",
});
const tarballsDir = assertSafeGeneratedPath(args.tarballs_dir);
const timeout = Number.parseInt(args.timeout_ms, 10);
const publishTimeout = Number.parseInt(args.publish_timeout_ms, 10);
const readbackAttempts = Number.parseInt(args.readback_attempts, 10);
const readbackInterval = Number.parseInt(args.readback_interval_ms, 10);
if (
  !Number.isSafeInteger(timeout) ||
  timeout <= 0 ||
  !Number.isSafeInteger(publishTimeout) ||
  publishTimeout <= 0 ||
  !Number.isSafeInteger(readbackAttempts) ||
  readbackAttempts <= 0 ||
  !Number.isSafeInteger(readbackInterval) ||
  readbackInterval <= 0
) {
  throw new Error("npm timeout and read-back options must be positive integers");
}
const manifest = JSON.parse(
  fs.readFileSync(path.join(tarballsDir, "publish-manifest.json"), "utf8"),
);

const release = releaseVersion(manifest.releaseTag);
if (release.version !== manifest.version) {
  throw new Error("publish manifest release tag and npm version do not match");
}
if (manifest.packages.length !== 7 || manifest.packages.at(-1)?.role !== "main") {
  throw new Error("publish manifest must contain six platform packages followed by the main package");
}
for (const [index, record] of manifest.packages.entries()) {
  const expectedRole = index === manifest.packages.length - 1 ? "main" : "platform";
  const expectedTag = expectedRole === "main" ? release.mainDistTag : "npm-bootstrap";
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

function registryMetadata(record, { freshCache = false } = {}) {
  const cacheDirectory = freshCache
    ? fs.mkdtempSync(path.join(os.tmpdir(), "loomloom-npm-registry-readback-"))
    : null;
  try {
    const commandArgs = ["view", `${record.name}@${record.version}`, "dist", "--json"];
    if (cacheDirectory) {
      commandArgs.push("--cache", cacheDirectory, "--prefer-online");
    }
    const result = npm(commandArgs, { capture: true });
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
  } finally {
    if (cacheDirectory) {
      fs.rmSync(cacheDirectory, { recursive: true, force: true });
    }
  }
}

function metadataMatches(record, metadata) {
  return metadata && metadata.integrity === record.integrity && metadata.shasum === record.shasum;
}

function sleep(milliseconds) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, milliseconds);
}

// waitForRegistryMetadata polls every pending record each round, so packages published together
// propagate in parallel instead of one Registry wait per package.
function waitForRegistryMetadata(records) {
  let pending = records;
  for (let attempt = 1; attempt <= readbackAttempts && pending.length > 0; attempt += 1) {
    const stillPending = [];
    for (const record of pending) {
      const metadata = registryMetadata(record, { freshCache: true });
      if (!metadata) {
        stillPending.push(record);
        continue;
      }
      if (!metadataMatches(record, metadata)) {
        throw new Error(
          `registry read-back did not match ${record.name}@${record.version}: ` +
          `expected integrity=${record.integrity} shasum=${record.shasum}, ` +
          `got integrity=${metadata.integrity} shasum=${metadata.shasum}`,
        );
      }
      console.log(`published and verified ${record.name}@${record.version}`);
    }
    pending = stillPending;
    if (pending.length > 0 && attempt < readbackAttempts) {
      console.log(
        `waiting for Registry metadata (${attempt}/${readbackAttempts}): ` +
        pending.map((record) => record.name).join(", "),
      );
      sleep(readbackInterval);
    }
  }
  if (pending.length > 0) {
    throw new Error(
      `registry read-back did not find ${pending.map((record) => `${record.name}@${record.version}`).join(", ")} ` +
      `after ${readbackAttempts} attempts`,
    );
  }
}

// publishGroup publishes the missing records of one group, then waits for all of them together.
async function publishGroup(records) {
  const missing = [];
  for (const record of records) {
    const existing = registryMetadata(record, { freshCache: true });
    if (existing) {
      if (!metadataMatches(record, existing)) {
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

    missing.push(record);
  }
  await runConcurrent(missing, 3, (record) => new Promise((resolve, reject) => {
    const child = spawn(npmCommand, [...npmCommandArgs, "publish", path.join(tarballsDir, record.tarball),
      "--access", "public", "--tag", record.publishTag, "--registry", args.registry],
    { stdio: "inherit", timeout: publishTimeout });
    let processError;
    child.once("error", (error) => { processError = error; });
    child.once("close", (code) => code === 0 && !processError
      ? resolve()
      : reject(new Error(`failed to publish ${record.name}@${record.version}`, { cause: processError })));
  }));
  waitForRegistryMetadata(missing);
}

// Platform packages first, all verified before the main package that depends on them is published.
await publishGroup(manifest.packages.filter((record) => record.role === "platform"));
await publishGroup(manifest.packages.filter((record) => record.role === "main"));

console.log(args.publish ? "npm publication completed" : "npm publication plan completed without writes");
