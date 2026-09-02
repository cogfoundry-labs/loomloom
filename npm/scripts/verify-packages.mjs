#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";
import {
  assertSafeGeneratedPath,
  npmCommand,
  npmCommandArgs,
  parseArgs,
  platforms,
  repoRoot,
  repository,
  sha256,
} from "./lib.mjs";

const args = parseArgs(process.argv.slice(2), {
  packages_dir: path.join(repoRoot, "dist", "npm"),
});
const packagesDir = assertSafeGeneratedPath(args.packages_dir);
const releaseManifestFile = path.join(packagesDir, "release-manifest.json");

if (!fs.existsSync(releaseManifestFile)) {
  throw new Error(`missing npm release manifest: ${releaseManifestFile}`);
}

const releaseManifest = JSON.parse(fs.readFileSync(releaseManifestFile, "utf8"));
const expectedRepository = JSON.stringify(repository);

function expectedFiles(record) {
  if (record.role === "main") {
    return new Set(["LICENSE", "README.md", "bin/loomloom.cjs", "package.json", "platforms.json"]);
  }
  const platform = platforms.find((candidate) => `${candidate.os}-${candidate.cpu}` === record.platform);
  if (!platform) {
    throw new Error(`unknown platform record: ${record.platform}`);
  }
  return new Set(["LICENSE", "README.md", platform.binary, "metadata.json", "package.json"]);
}

function assertPackageMetadata(record, packageDir, packageJSON) {
  if (packageJSON.name !== record.name || packageJSON.version !== record.version) {
    throw new Error(`package identity mismatch in ${packageDir}`);
  }
  if (packageJSON.license !== "Apache-2.0") {
    throw new Error(`package license mismatch in ${packageDir}`);
  }
  if (JSON.stringify(packageJSON.repository) !== expectedRepository) {
    throw new Error(`package repository must exactly match ${repository.url}: ${packageDir}`);
  }
  if (packageJSON.publishConfig?.access !== "public") {
    throw new Error(`package publishConfig.access must be public: ${packageDir}`);
  }
  if (!Array.isArray(packageJSON.files) || packageJSON.files.length === 0) {
    throw new Error(`package files allowlist is required: ${packageDir}`);
  }
}

function verifyPlatformPackage(record, packageDir, packageJSON) {
  const metadata = JSON.parse(fs.readFileSync(path.join(packageDir, "metadata.json"), "utf8"));
  const platform = platforms.find((candidate) => candidate.package === record.name);
  if (!platform) {
    throw new Error(`unexpected platform package: ${record.name}`);
  }
  if (packageJSON.os?.[0] !== platform.os || packageJSON.cpu?.[0] !== platform.cpu) {
    throw new Error(`os/cpu mismatch for ${record.name}`);
  }
  const binary = path.join(packageDir, metadata.binary);
  if (metadata.binary !== platform.binary || sha256(binary) !== metadata.binarySHA256) {
    throw new Error(`binary SHA-256 mismatch for ${record.name}`);
  }
  return metadata;
}

function verifyMainPackage(record, packageDir, packageJSON, platformMetadata) {
  if (packageJSON.bin?.loomloom !== "bin/loomloom.cjs") {
    throw new Error("main package must expose bin/loomloom.cjs as loomloom");
  }
  const manifest = JSON.parse(fs.readFileSync(path.join(packageDir, "platforms.json"), "utf8"));
  for (const platform of platforms) {
    const key = `${platform.os}-${platform.cpu}`;
    if (packageJSON.optionalDependencies?.[platform.package] !== record.version) {
      throw new Error(`main package must pin ${platform.package}@${record.version}`);
    }
    const selected = manifest.platforms?.[key];
    const metadata = platformMetadata.get(platform.package);
    if (!selected || !metadata || selected.binarySHA256 !== metadata.binarySHA256) {
      throw new Error(`main package SHA-256 is not frozen for ${key}`);
    }
  }
}

function verifyPackContents(record, packageDir) {
  const output = execFileSync(npmCommand, [...npmCommandArgs, "pack", "--dry-run", "--json"], {
    cwd: packageDir,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  const result = JSON.parse(output);
  const files = new Set(result[0].files.map((entry) => entry.path));
  const expected = expectedFiles(record);
  if (files.size !== expected.size || [...expected].some((file) => !files.has(file))) {
    throw new Error(
      `unexpected npm package contents for ${record.name}: ${[...files].sort().join(", ")}`,
    );
  }
}

const platformMetadata = new Map();
for (const record of releaseManifest.packages.filter((candidate) => candidate.role === "platform")) {
  const packageDir = path.join(packagesDir, record.directory);
  const packageJSON = JSON.parse(fs.readFileSync(path.join(packageDir, "package.json"), "utf8"));
  assertPackageMetadata(record, packageDir, packageJSON);
  platformMetadata.set(record.name, verifyPlatformPackage(record, packageDir, packageJSON));
  verifyPackContents(record, packageDir);
}

const mainRecord = releaseManifest.packages.find((record) => record.role === "main");
if (!mainRecord) {
  throw new Error("npm release manifest is missing the main package");
}
const mainDir = path.join(packagesDir, mainRecord.directory);
const mainJSON = JSON.parse(fs.readFileSync(path.join(mainDir, "package.json"), "utf8"));
assertPackageMetadata(mainRecord, mainDir, mainJSON);
verifyMainPackage(mainRecord, mainDir, mainJSON, platformMetadata);
verifyPackContents(mainRecord, mainDir);

console.log(`verified ${releaseManifest.packages.length} npm packages in ${packagesDir}`);
