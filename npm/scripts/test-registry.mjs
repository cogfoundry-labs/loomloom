#!/usr/bin/env node

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { startRegistry } from "../test/fixtures/registry.mjs";
import {
  assertSafeGeneratedPath,
  npmCommand,
  npmCommandArgs,
  npmRoot,
  npxCommand,
  npxCommandArgs,
  parseArgs,
  requiresCommandShell,
  repoRoot,
} from "./lib.mjs";

const args = parseArgs(process.argv.slice(2), {
  tarballs_dir: path.join(repoRoot, "dist", "npm-tarballs"),
});
const tarballsDir = assertSafeGeneratedPath(args.tarballs_dir);
const manifest = JSON.parse(
  fs.readFileSync(path.join(tarballsDir, "publish-manifest.json"), "utf8"),
);
const testRoot = fs.mkdtempSync(path.join(os.tmpdir(), "loomloom-npm-registry-"));
const userConfig = path.join(testRoot, "npmrc");
const npmEnv = {
  ...process.env,
  NPM_CONFIG_USERCONFIG: userConfig,
  npm_config_cache: path.join(testRoot, "cache"),
};
let registryFixture;

function run(command, commandArgs, options = {}) {
  const result = spawnSync(command, commandArgs, {
    cwd: options.cwd ?? testRoot,
    env: npmEnv,
    encoding: "utf8",
    stdio: options.capture ? ["ignore", "pipe", "pipe"] : "inherit",
    shell: requiresCommandShell(command),
  });
  if (result.status !== (options.expectedStatus ?? 0)) {
    const details = `${result.stdout ?? ""}\n${result.stderr ?? ""}`.trim();
    throw new Error(
      `${command} ${commandArgs.join(" ")} exited ${result.status}; expected ${options.expectedStatus ?? 0}\n${details}`,
    );
  }
  return result;
}

function installedCommand(prefix) {
  return process.platform === "win32"
    ? path.join(prefix, "loomloom.cmd")
    : path.join(prefix, "bin", "loomloom");
}

function localCommand(prefix) {
  return process.platform === "win32"
    ? path.join(prefix, "node_modules", ".bin", "loomloom.cmd")
    : path.join(prefix, "node_modules", ".bin", "loomloom");
}

try {
  const mainIndex = manifest.packages.findIndex((record) => record.role === "main");
  if (mainIndex !== manifest.packages.length - 1) {
    throw new Error("publish manifest must place the main package last");
  }
  if (manifest.packages.slice(0, -1).some((record) => record.role !== "platform")) {
    throw new Error("all platform packages must precede the main package");
  }
  const mainPackage = manifest.packages.at(-1);
  const mainSpecifier = mainPackage.publishTag === "latest"
    ? "@cogfoundry/loomloom"
    : `@cogfoundry/loomloom@${mainPackage.publishTag}`;

  registryFixture = await startRegistry(manifest.packages, tarballsDir);
  const { url: registry, token } = registryFixture;
  fs.writeFileSync(
    userConfig,
    `registry=${registry}\n${registry.replace(/^https?:/, "")}/:_authToken=${token}\n`,
    { mode: 0o600 },
  );
  npmEnv.npm_config_registry = registry;

  const firstPlatform = manifest.packages[0];
  run(
    npmCommand,
    [
      ...npmCommandArgs,
      "publish",
      path.join(tarballsDir, firstPlatform.tarball),
      "--registry",
      registry,
      "--access",
      "public",
      "--tag",
      firstPlatform.publishTag,
      "--ignore-scripts",
    ],
  );

  const publishScript = path.join(npmRoot, "scripts", "publish-packages.mjs");
  const publishArgs = [
    publishScript,
    "--tarballs-dir",
    tarballsDir,
    "--registry",
    registry,
    "--publish",
    "--confirm-tag",
    manifest.releaseTag,
  ];
  run(process.execPath, publishArgs);
  run(process.execPath, publishArgs);

  const prefix = path.join(testRoot, "global");
  run(npmCommand, [
    ...npmCommandArgs,
    "install",
    "--global",
    mainSpecifier,
    "--registry",
    registry,
    "--prefix",
    prefix,
    "--ignore-scripts",
  ]);
  const loomloom = installedCommand(prefix);
  run(loomloom, ["--version"]);
  run(loomloom, ["--help"]);
  const invalid = run(loomloom, ["definitely-not-a-command"], {
    capture: true,
    expectedStatus: 1,
  });
  if (!/unknown command/.test(invalid.stderr)) {
    throw new Error(`launcher did not preserve the CLI failure path: ${invalid.stderr}`);
  }

  run(npmCommand, [
    ...npmCommandArgs,
    "exec",
    "--yes",
    "--registry",
    registry,
    `--package=${mainSpecifier}`,
    "--",
    "loomloom",
    "--help",
  ]);
  run(npxCommand, [
    ...npxCommandArgs,
    "--yes",
    "--registry",
    registry,
    mainSpecifier,
    "--version",
  ]);

  const omittedPrefix = path.join(testRoot, "omitted-optional");
  run(npmCommand, [
    ...npmCommandArgs,
    "install",
    mainSpecifier,
    "--registry",
    registry,
    "--prefix",
    omittedPrefix,
    "--omit=optional",
    "--ignore-scripts",
  ]);
  const omitted = run(localCommand(omittedPrefix), ["--version"], {
    capture: true,
    expectedStatus: 1,
  });
  if (!/required platform package .* is missing/.test(omitted.stderr)) {
    throw new Error(`omitted optional dependency did not fail explicitly: ${omitted.stderr}`);
  }

  run(npmCommand, [
    ...npmCommandArgs,
    "uninstall",
    "--global",
    "@cogfoundry/loomloom",
    "--registry",
    registry,
    "--prefix",
    prefix,
    "--ignore-scripts",
  ]);
  if (fs.existsSync(loomloom)) {
    throw new Error(`global uninstall left command behind: ${loomloom}`);
  }

  console.log(`verified npm install, npm exec, npx, omitted optional dependency, and uninstall via ${registry}`);
} finally {
  await registryFixture?.close();
  fs.rmSync(testRoot, { recursive: true, force: true });
}
