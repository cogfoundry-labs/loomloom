#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";
import {
  assertSafeGeneratedPath,
  npmCommand,
  npmCommandArgs,
  npmRoot,
  parseArgs,
  repoRoot,
  sha1,
  sha512Integrity,
  writeJSON,
} from "./lib.mjs";

const args = parseArgs(process.argv.slice(2), {
  packages_dir: path.join(repoRoot, "dist", "npm"),
  out_dir: path.join(repoRoot, "dist", "npm-tarballs"),
});
const packagesDir = assertSafeGeneratedPath(args.packages_dir);
const outDir = assertSafeGeneratedPath(args.out_dir);
const releaseManifest = JSON.parse(
  fs.readFileSync(path.join(packagesDir, "release-manifest.json"), "utf8"),
);

execFileSync(
  process.execPath,
  [path.join(npmRoot, "scripts", "verify-packages.mjs"), "--packages-dir", packagesDir],
  { stdio: "inherit" },
);

fs.rmSync(outDir, { recursive: true, force: true });
fs.mkdirSync(outDir, { recursive: true });

const packed = [];
for (const record of releaseManifest.packages) {
  const packageDir = path.join(packagesDir, record.directory);
  const output = execFileSync(
    npmCommand,
    [...npmCommandArgs, "pack", "--json", "--pack-destination", outDir],
    {
      cwd: packageDir,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    },
  );
  const result = JSON.parse(output)[0];
  const tarball = path.join(outDir, result.filename);
  packed.push({
    ...record,
    tarball: result.filename,
    integrity: sha512Integrity(tarball),
    shasum: sha1(tarball),
  });
}

writeJSON(path.join(outDir, "publish-manifest.json"), {
  releaseTag: releaseManifest.releaseTag,
  version: releaseManifest.version,
  packages: packed,
});

console.log(`packed ${packed.length} npm tarballs in ${outDir}`);
