#!/usr/bin/env node
"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");
const { spawn } = require("node:child_process");

const DEFAULT_MANIFEST = path.join(__dirname, "..", "platforms.json");
const FORWARDED_SIGNALS = process.platform === "win32"
  ? ["SIGINT", "SIGTERM", "SIGBREAK"]
  : ["SIGINT", "SIGTERM", "SIGHUP"];

function platformKey(platform = process.platform, arch = process.arch) {
  return `${platform}-${arch}`;
}

function loadManifest(manifestPath = DEFAULT_MANIFEST) {
  return JSON.parse(fs.readFileSync(manifestPath, "utf8"));
}

function selectPlatform(manifest, platform = process.platform, arch = process.arch) {
  const key = platformKey(platform, arch);
  const selected = manifest.platforms[key];
  if (!selected) {
    const supported = Object.keys(manifest.platforms).sort().join(", ");
    throw new Error(
      `unsupported platform ${key}; supported platforms: ${supported}`,
    );
  }
  return { key, ...selected };
}

function locateBinary(selected, resolve = require.resolve) {
  let packageJSON;
  try {
    packageJSON = resolve(`${selected.package}/package.json`);
  } catch (error) {
    if (error && error.code === "MODULE_NOT_FOUND") {
      throw new Error(
        `required platform package ${selected.package} is missing; reinstall @cogfoundry/loomloom without --omit=optional`,
      );
    }
    throw error;
  }
  const binary = path.join(path.dirname(packageJSON), selected.binary);
  if (!fs.existsSync(binary)) {
    throw new Error(
      `platform package ${selected.package} does not contain ${selected.binary}`,
    );
  }
  return binary;
}

function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

function verifyBinary(binary, expectedSHA256) {
  const actualSHA256 = sha256(binary);
  if (actualSHA256 !== expectedSHA256) {
    throw new Error(
      `binary checksum mismatch for ${binary}; expected ${expectedSHA256}, got ${actualSHA256}`,
    );
  }
}

function execute(binary, args = process.argv.slice(2)) {
  const child = spawn(binary, args, {
    cwd: process.cwd(),
    env: process.env,
    stdio: "inherit",
    windowsHide: false,
  });

  const handlers = new Map();
  for (const signal of FORWARDED_SIGNALS) {
    const handler = () => {
      if (!child.killed) {
        try {
          child.kill(signal);
        } catch (error) {
          console.error(`loomloom: failed to forward ${signal}: ${error.message}`);
        }
      }
    };
    handlers.set(signal, handler);
    process.on(signal, handler);
  }

  function removeSignalHandlers() {
    for (const [signal, handler] of handlers) {
      process.off(signal, handler);
    }
  }

  child.on("error", (error) => {
    removeSignalHandlers();
    console.error(`loomloom: failed to start ${binary}: ${error.message}`);
    process.exitCode = 1;
  });

  child.on("exit", (code, signal) => {
    removeSignalHandlers();
    if (signal) {
      try {
        process.kill(process.pid, signal);
      } catch {
        process.exitCode = 1;
      }
      return;
    }
    process.exitCode = code === null ? 1 : code;
  });
}

function run() {
  const manifest = loadManifest();
  const selected = selectPlatform(manifest);
  const binary = locateBinary(selected);
  verifyBinary(binary, selected.binarySHA256);
  execute(binary);
}

if (require.main === module) {
  try {
    run();
  } catch (error) {
    console.error(`loomloom: ${error.message}`);
    process.exitCode = 1;
  }
}

module.exports = {
  execute,
  loadManifest,
  locateBinary,
  platformKey,
  run,
  selectPlatform,
  sha256,
  verifyBinary,
};
