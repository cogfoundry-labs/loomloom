import assert from "node:assert/strict";
import crypto from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const launcher = require("../launcher/loomloom.cjs");

test("selectPlatform maps Node platform and architecture", () => {
  const manifest = {
    platforms: {
      "darwin-arm64": {
        package: "@cogfoundry/loomloom-darwin-arm64",
        binary: "bin/loomloom",
        binarySHA256: "a".repeat(64),
      },
    },
  };

  assert.deepEqual(launcher.selectPlatform(manifest, "darwin", "arm64"), {
    key: "darwin-arm64",
    package: "@cogfoundry/loomloom-darwin-arm64",
    binary: "bin/loomloom",
    binarySHA256: "a".repeat(64),
  });
});

test("selectPlatform rejects unsupported platforms with the support matrix", () => {
  const manifest = { platforms: { "linux-x64": {} } };
  assert.throws(
    () => launcher.selectPlatform(manifest, "freebsd", "x64"),
    /unsupported platform freebsd-x64; supported platforms: linux-x64/,
  );
});

test("locateBinary reports omitted optional dependency", () => {
  const missing = new Error("not found");
  missing.code = "MODULE_NOT_FOUND";
  assert.throws(
    () =>
      launcher.locateBinary(
        {
          package: "@cogfoundry/loomloom-linux-x64",
          binary: "bin/loomloom",
        },
        () => {
          throw missing;
        },
      ),
    /reinstall @cogfoundry\/loomloom without --omit=optional/,
  );
});

test("verifyBinary accepts the frozen SHA and rejects tampering", () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "loomloom-launcher-test-"));
  const binary = path.join(directory, "loomloom");
  fs.writeFileSync(binary, "verified binary");
  const expected = crypto.createHash("sha256").update("verified binary").digest("hex");

  try {
    launcher.verifyBinary(binary, expected);
    fs.appendFileSync(binary, " tampered");
    assert.throws(() => launcher.verifyBinary(binary, expected), /binary checksum mismatch/);
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});
