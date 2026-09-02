import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const npmRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
export const repoRoot = path.resolve(npmRoot, "..");
export const npmCommand = process.execPath;
export const npmCommandArgs = [path.join(npmRoot, "node_modules", "npm", "bin", "npm-cli.js")];
export const npxCommand = process.execPath;
export const npxCommandArgs = [path.join(npmRoot, "node_modules", "npm", "bin", "npx-cli.js")];
export const repository = {
  type: "git",
  url: "git+https://github.com/cogfoundry-labs/loomloom.git",
};

export const platforms = [
  {
    os: "linux",
    cpu: "x64",
    package: "@cogfoundry/loomloom-linux-x64",
    archive: "loomloom-linux-amd64.tar.gz",
    binary: "bin/loomloom",
  },
  {
    os: "linux",
    cpu: "arm64",
    package: "@cogfoundry/loomloom-linux-arm64",
    archive: "loomloom-linux-arm64.tar.gz",
    binary: "bin/loomloom",
  },
  {
    os: "darwin",
    cpu: "x64",
    package: "@cogfoundry/loomloom-darwin-x64",
    archive: "loomloom-darwin-amd64.tar.gz",
    binary: "bin/loomloom",
  },
  {
    os: "darwin",
    cpu: "arm64",
    package: "@cogfoundry/loomloom-darwin-arm64",
    archive: "loomloom-darwin-arm64.tar.gz",
    binary: "bin/loomloom",
  },
  {
    os: "win32",
    cpu: "x64",
    package: "@cogfoundry/loomloom-win32-x64",
    archive: "loomloom-windows-amd64.zip",
    binary: "bin/loomloom.exe",
  },
  {
    os: "win32",
    cpu: "arm64",
    package: "@cogfoundry/loomloom-win32-arm64",
    archive: "loomloom-windows-arm64.zip",
    binary: "bin/loomloom.exe",
  },
];

export function parseArgs(argv, defaults = {}) {
  const result = { ...defaults };
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (!argument.startsWith("--")) {
      throw new Error(`unexpected argument: ${argument}`);
    }
    const key = argument.slice(2).replaceAll("-", "_");
    if (key === "publish") {
      result.publish = true;
      continue;
    }
    const value = argv[index + 1];
    if (!value || value.startsWith("--")) {
      throw new Error(`${argument} requires a value`);
    }
    result[key] = value;
    index += 1;
  }
  return result;
}

export function betaVersion(tag) {
  const match = /^v(\d+\.\d+\.\d+-beta\.\d+)$/.exec(tag);
  if (!match) {
    throw new Error(`unsupported npm release tag ${tag}; expected vX.Y.Z-beta.N`);
  }
  return match[1];
}

export function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

export function sha512Integrity(file) {
  const digest = crypto.createHash("sha512").update(fs.readFileSync(file)).digest("base64");
  return `sha512-${digest}`;
}

export function sha1(file) {
  return crypto.createHash("sha1").update(fs.readFileSync(file)).digest("hex");
}

export function writeJSON(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
}

export function assertSafeGeneratedPath(target) {
  const absolute = path.resolve(target);
  const relative = path.relative(repoRoot, absolute);
  if (!relative || relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new Error(`generated output must be a child of the repository: ${absolute}`);
  }
  return absolute;
}

export function packageDirectoryName(packageName) {
  return packageName.replace("@cogfoundry/", "");
}
