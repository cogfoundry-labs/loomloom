#!/usr/bin/env node

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import {
  assertSafeGeneratedPath,
  betaVersion,
  npmRoot,
  packageDirectoryName,
  parseArgs,
  platforms,
  repoRoot,
  repository,
  sha256,
  writeJSON,
} from "./lib.mjs";

const args = parseArgs(process.argv.slice(2), {
  assets_dir: path.join(repoRoot, "release"),
  out_dir: path.join(repoRoot, "dist", "npm"),
});

if (!args.tag) {
  throw new Error("--tag is required");
}

const version = betaVersion(args.tag);
const assetsDir = path.resolve(args.assets_dir);
const outDir = assertSafeGeneratedPath(args.out_dir);
const checksumsFile = path.join(assetsDir, "checksums.txt");
const licenseFile = path.join(repoRoot, "LICENSE");
const launcherFile = path.join(npmRoot, "launcher", "loomloom.cjs");

function readChecksums(file) {
  const checksums = new Map();
  for (const line of fs.readFileSync(file, "utf8").split(/\r?\n/)) {
    if (!line.trim()) {
      continue;
    }
    const match = /^([0-9a-f]{64})\s+[ *]?([^/]+)$/.exec(line.trim());
    if (!match) {
      throw new Error(`invalid checksums.txt entry: ${line}`);
    }
    if (checksums.has(match[2])) {
      throw new Error(`duplicate checksums.txt entry: ${match[2]}`);
    }
    checksums.set(match[2], match[1]);
  }
  return checksums;
}

function verifyArchive(platform, checksums) {
  const archive = path.join(assetsDir, platform.archive);
  const expected = checksums.get(platform.archive);
  if (!expected) {
    throw new Error(`checksums.txt does not list ${platform.archive}`);
  }
  if (!fs.existsSync(archive)) {
    throw new Error(`release asset is missing: ${archive}`);
  }
  const actual = sha256(archive);
  if (actual !== expected) {
    throw new Error(
      `release asset checksum mismatch for ${platform.archive}; expected ${expected}, got ${actual}`,
    );
  }
  return { archive, archiveSHA256: expected };
}

function extractBinary(platform, archive, destination) {
  const extractDir = fs.mkdtempSync(path.join(os.tmpdir(), "loomloom-npm-extract-"));
  try {
    if (platform.archive.endsWith(".tar.gz")) {
      execFileSync("tar", ["-xzf", archive, "-C", extractDir], { stdio: "pipe" });
    } else if (platform.archive.endsWith(".zip")) {
      execFileSync("unzip", ["-q", archive, "-d", extractDir], { stdio: "pipe" });
    } else {
      throw new Error(`unsupported release archive: ${platform.archive}`);
    }
    const sourceName = platform.os === "win32" ? "loomloom.exe" : "loomloom";
    const source = path.join(extractDir, sourceName);
    if (!fs.existsSync(source)) {
      throw new Error(`${platform.archive} does not contain ${sourceName} at its root`);
    }
    fs.mkdirSync(path.dirname(destination), { recursive: true });
    fs.copyFileSync(source, destination);
    if (platform.os !== "win32") {
      fs.chmodSync(destination, 0o755);
    }
  } finally {
    fs.rmSync(extractDir, { recursive: true, force: true });
  }
}

function copyCommonFiles(packageDir) {
  fs.copyFileSync(licenseFile, path.join(packageDir, "LICENSE"));
}

function platformReadme(platform) {
  return `# ${platform.package}\n\n` +
    `Platform binary for [@cogfoundry/loomloom](https://www.npmjs.com/package/@cogfoundry/loomloom).\n\n` +
    `This package is installed automatically for ${platform.os}/${platform.cpu}. Do not depend on it directly.\n`;
}

function mainReadme() {
  return `# @cogfoundry/loomloom\n\n` +
    `npm distribution for the LoomLoom Go CLI.\n\n` +
    `\`\`\`sh\n` +
    `npm install -g @cogfoundry/loomloom@beta\n` +
    `loomloom --version\n` +
    `\`\`\`\n\n` +
    `GitHub Release assets remain the binary source of truth. The launcher selects and verifies one platform package without downloading a binary during npm lifecycle scripts.\n`;
}

if (!fs.existsSync(checksumsFile)) {
  throw new Error(`missing release checksums: ${checksumsFile}`);
}
if (!fs.existsSync(licenseFile) || !fs.existsSync(launcherFile)) {
  throw new Error("repository LICENSE or npm launcher source is missing");
}

const checksums = readChecksums(checksumsFile);
fs.rmSync(outDir, { recursive: true, force: true });
fs.mkdirSync(outDir, { recursive: true });

const mainPlatforms = {};
const packageRecords = [];

for (const platform of platforms) {
  const { archive, archiveSHA256 } = verifyArchive(platform, checksums);
  const packageDir = path.join(outDir, packageDirectoryName(platform.package));
  const binary = path.join(packageDir, platform.binary);
  fs.mkdirSync(packageDir, { recursive: true });
  extractBinary(platform, archive, binary);
  const binarySHA256 = sha256(binary);

  writeJSON(path.join(packageDir, "package.json"), {
    name: platform.package,
    version,
    description: `LoomLoom CLI binary for ${platform.os}/${platform.cpu}`,
    license: "Apache-2.0",
    repository,
    os: [platform.os],
    cpu: [platform.cpu],
    files: ["bin", "metadata.json", "README.md", "LICENSE"],
    publishConfig: { access: "public" },
  });
  writeJSON(path.join(packageDir, "metadata.json"), {
    releaseTag: args.tag,
    sourceRepository: "https://github.com/cogfoundry-labs/loomloom",
    archive: platform.archive,
    archiveSHA256,
    binary: platform.binary,
    binarySHA256,
  });
  fs.writeFileSync(path.join(packageDir, "README.md"), platformReadme(platform));
  copyCommonFiles(packageDir);

  const nodeKey = `${platform.os}-${platform.cpu}`;
  mainPlatforms[nodeKey] = {
    package: platform.package,
    binary: platform.binary,
    binarySHA256,
    archive: platform.archive,
    archiveSHA256,
  };
  packageRecords.push({
    name: platform.package,
    version,
    role: "platform",
    platform: nodeKey,
    directory: packageDirectoryName(platform.package),
    publishTag: "npm-bootstrap",
  });
}

const mainPackageDir = path.join(outDir, "loomloom");
fs.mkdirSync(path.join(mainPackageDir, "bin"), { recursive: true });
fs.copyFileSync(launcherFile, path.join(mainPackageDir, "bin", "loomloom.cjs"));
fs.chmodSync(path.join(mainPackageDir, "bin", "loomloom.cjs"), 0o755);
writeJSON(path.join(mainPackageDir, "platforms.json"), {
  releaseTag: args.tag,
  version,
  platforms: mainPlatforms,
});
writeJSON(path.join(mainPackageDir, "package.json"), {
  name: "@cogfoundry/loomloom",
  version,
  description: "LoomLoom CLI for defining, compiling, executing, and managing AI work as software",
  license: "Apache-2.0",
  repository,
  bin: { loomloom: "bin/loomloom.cjs" },
  engines: { node: ">=18" },
  files: ["bin", "platforms.json", "README.md", "LICENSE"],
  optionalDependencies: Object.fromEntries(
    platforms.map((platform) => [platform.package, version]),
  ),
  publishConfig: { access: "public" },
});
fs.writeFileSync(path.join(mainPackageDir, "README.md"), mainReadme());
copyCommonFiles(mainPackageDir);
packageRecords.push({
  name: "@cogfoundry/loomloom",
  version,
  role: "main",
  directory: "loomloom",
  publishTag: "beta",
});

writeJSON(path.join(outDir, "release-manifest.json"), {
  releaseTag: args.tag,
  version,
  sourceChecksums: path.relative(repoRoot, checksumsFile),
  packages: packageRecords,
});

console.log(`prepared ${packageRecords.length} npm packages in ${outDir}`);
