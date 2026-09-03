import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
import {
  assertSafeGeneratedPath,
  releaseVersion,
  repoRoot,
  requiresCommandShell,
} from "../scripts/lib.mjs";

test("releaseVersion classifies stable and Beta release channels", () => {
  assert.deepEqual(releaseVersion("v1.2.3"), {
    version: "1.2.3",
    channel: "stable",
    mainDistTag: "latest",
  });
  assert.deepEqual(releaseVersion("v1.2.3-beta.4"), {
    version: "1.2.3-beta.4",
    channel: "beta",
    mainDistTag: "beta",
  });
  for (const tag of ["v1.2.3-rc.1", "1.2.3-beta.1", "v1.2-beta.1"]) {
    assert.throws(() => releaseVersion(tag), /expected vX\.Y\.Z or vX\.Y\.Z-beta\.N/);
  }
});

test("generated outputs must stay below the repository root", () => {
  assert.equal(
    assertSafeGeneratedPath(path.join(repoRoot, "dist", "npm")),
    path.join(repoRoot, "dist", "npm"),
  );
  assert.throws(() => assertSafeGeneratedPath(repoRoot), /must be a child/);
  assert.throws(() => assertSafeGeneratedPath(path.dirname(repoRoot)), /must be a child/);
});

test("Windows command shims use the system shell", () => {
  assert.equal(requiresCommandShell("C:\\tmp\\loomloom.cmd", "win32"), true);
  assert.equal(requiresCommandShell("C:\\tmp\\loomloom.BAT", "win32"), true);
  assert.equal(requiresCommandShell("C:\\tmp\\loomloom.exe", "win32"), false);
  assert.equal(requiresCommandShell("/tmp/loomloom.cmd", "linux"), false);
});

test("Windows executes .cmd shims through the selected shell", {
  skip: process.platform !== "win32",
}, () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "loomloom-cmd-test-"));
  const shim = path.join(directory, "loomloom.cmd");
  fs.writeFileSync(
    shim,
    "@echo off\r\nif \"%1\"==\"--version\" (exit /b 0) else (exit /b 9)\r\n",
  );
  try {
    const result = spawnSync(shim, ["--version"], {
      shell: requiresCommandShell(shim),
    });
    assert.equal(result.status, 0);
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});
