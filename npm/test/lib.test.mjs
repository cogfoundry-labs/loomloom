import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import {
  assertSafeGeneratedPath,
  betaVersion,
  repoRoot,
} from "../scripts/lib.mjs";

test("betaVersion accepts only the Beta release channel", () => {
  assert.equal(betaVersion("v1.2.3-beta.4"), "1.2.3-beta.4");
  for (const tag of ["v1.2.3", "v1.2.3-rc.1", "1.2.3-beta.1", "v1.2-beta.1"]) {
    assert.throws(() => betaVersion(tag), /expected vX\.Y\.Z-beta\.N/);
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
