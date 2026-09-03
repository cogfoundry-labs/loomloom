#!/usr/bin/env node

import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { spawn, spawnSync } from "node:child_process";
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
const configFile = path.join(testRoot, "verdaccio.yaml");
let registryProcess;
let registryOutput = "";

function run(command, commandArgs, options = {}) {
  const result = spawnSync(command, commandArgs, {
    cwd: options.cwd,
    env: options.env ?? process.env,
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

async function freePort() {
  return new Promise((resolve, reject) => {
    const server = http.createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      server.close(() => resolve(address.port));
    });
  });
}

async function waitForRegistry(registry) {
  let lastError;
  for (let attempt = 0; attempt < 60; attempt += 1) {
    try {
      const response = await fetch(`${registry}/-/ping`);
      if (response.ok) {
        return;
      }
      lastError = new Error(`HTTP ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`Verdaccio did not become ready: ${lastError?.message}`);
}

async function createLocalUser(registry) {
  const username = "loomloom-test";
  const password = "local-only";
  const email = "loomloom-test@example.invalid";
  const response = await fetch(
    `${registry}/-/user/org.couchdb.user:${encodeURIComponent(username)}`,
    {
      method: "PUT",
      headers: {
        Authorization: `Basic ${Buffer.from(`${username}:${password}`).toString("base64")}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        _id: `org.couchdb.user:${username}`,
        name: username,
        password,
        email,
        type: "user",
        roles: [],
        date: new Date().toISOString(),
      }),
    },
  );
  const result = await response.json();
  if (response.status !== 201 || typeof result.token !== "string" || !result.token) {
    throw new Error(`local Registry user creation failed with HTTP ${response.status}`);
  }
  return result.token;
}

async function stopRegistry() {
  if (!registryProcess || registryProcess.exitCode !== null) {
    return;
  }
  const exited = new Promise((resolve) => registryProcess.once("exit", resolve));
  registryProcess.kill("SIGTERM");
  await Promise.race([
    exited,
    new Promise((resolve) => setTimeout(resolve, 5_000)),
  ]);
  if (registryProcess.exitCode === null) {
    registryProcess.kill("SIGKILL");
    await exited;
  }
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

  const port = await freePort();
  const registry = `http://127.0.0.1:${port}`;
  fs.writeFileSync(
    configFile,
    [
      `storage: ${JSON.stringify(path.join(testRoot, "storage"))}`,
      "max_body_size: 30mb",
      "auth:",
      "  htpasswd:",
      `    file: ${JSON.stringify(path.join(testRoot, "htpasswd"))}`,
      "uplinks:",
      "packages:",
      "  '@*/*':",
      "    access: $all",
      "    publish: $authenticated",
      "    unpublish: $authenticated",
      "  '**':",
      "    access: $all",
      "    publish: $authenticated",
      "    unpublish: $authenticated",
      "security:",
      "  api:",
      "    jwt:",
      "      sign:",
      "        expiresIn: 1h",
      "",
    ].join("\n"),
  );

  registryProcess = spawn(
    process.execPath,
    [
      path.join(npmRoot, "node_modules", "verdaccio", "bin", "verdaccio"),
      "--config",
      configFile,
      "--listen",
      `127.0.0.1:${port}`,
    ],
    { cwd: testRoot, stdio: ["ignore", "pipe", "pipe"] },
  );
  registryProcess.stdout.on("data", (chunk) => { registryOutput += chunk; });
  registryProcess.stderr.on("data", (chunk) => { registryOutput += chunk; });
  await waitForRegistry(registry);

  const token = await createLocalUser(registry);
  if (!token || /\s/.test(token)) {
    throw new Error("local Registry authentication did not return one token");
  }
  const userConfig = path.join(testRoot, "npmrc");
  fs.writeFileSync(
    userConfig,
    `registry=${registry}\n${registry.replace(/^https?:/, "")}/:_authToken=${token}\n`,
    { mode: 0o600 },
  );
  const npmEnv = {
    ...process.env,
    NPM_CONFIG_USERCONFIG: userConfig,
    npm_config_registry: registry,
  };

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
    { env: npmEnv },
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
  run(process.execPath, publishArgs, { env: npmEnv });
  run(process.execPath, publishArgs, { env: npmEnv });

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
} catch (error) {
  if (registryProcess && registryProcess.exitCode !== 0) {
    console.error(`Verdaccio output:\n${registryOutput ?? ""}`);
  }
  throw error;
} finally {
  await stopRegistry();
  fs.rmSync(testRoot, { recursive: true, force: true });
}
