import assert from "node:assert/strict";
import crypto from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { startRegistry } from "./fixtures/registry.mjs";

function packageFixture(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "loomloom-registry-test-"));
  const bytes = Buffer.from("immutable fixture tarball");
  const record = {
    name: "@cogfoundry/loomloom",
    version: "1.2.3-beta.1",
    publishTag: "beta",
    tarball: "loomloom-1.2.3-beta.1.tgz",
    integrity: `sha512-${crypto.createHash("sha512").update(bytes).digest("base64")}`,
    shasum: crypto.createHash("sha1").update(bytes).digest("hex"),
  };
  fs.writeFileSync(path.join(directory, record.tarball), bytes);
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return { directory, bytes, record };
}

test("Registry fixture rejects tarballs that do not match the frozen manifest", async (t) => {
  const { directory, record } = packageFixture(t);
  await assert.rejects(
    startRegistry([{ ...record, shasum: "incorrect" }], directory),
    /tarball integrity mismatch/,
  );
});

test("Registry fixture checks authentication, immutable uploads and local read-back", async (t) => {
  const { directory, bytes, record } = packageFixture(t);
  const registry = await startRegistry([record], directory);
  t.after(() => registry.close());
  assert.equal(new URL(registry.url).hostname, "127.0.0.1");
  const url = `${registry.url}/${encodeURIComponent(record.name)}`;
  const body = {
    name: record.name,
    "dist-tags": { [record.publishTag]: record.version },
    versions: { [record.version]: {
      name: record.name,
      version: record.version,
      dist: { integrity: record.integrity, shasum: record.shasum, tarball: "https://example.invalid/package.tgz" },
    } },
    _attachments: { [record.tarball]: { data: bytes.toString("base64") } },
  };
  const publish = (data, authenticated = true) => fetch(url, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      ...(authenticated ? { Authorization: `Bearer ${registry.token}` } : {}),
    },
    body: JSON.stringify(data),
  });
  assert.equal((await fetch(url)).status, 404);
  assert.equal((await publish(body, false)).status, 401);
  assert.equal((await publish({ padding: "x".repeat(2 * 1024 * 1024) })).status, 400);
  assert.equal((await fetch(url, {
    method: "PUT",
    headers: { Authorization: `Bearer ${registry.token}` },
    body: "{invalid json",
  })).status, 400);
  const corrupt = structuredClone(body);
  corrupt._attachments[record.tarball].data = Buffer.from("modified").toString("base64");
  assert.equal((await publish(corrupt)).status, 400);
  assert.equal((await fetch(url)).status, 404);
  const concurrent = await Promise.all([publish(body), publish(body)]);
  assert.deepEqual(concurrent.map((response) => response.status).sort(), [201, 409]);
  assert.equal((await publish(body)).status, 409);
  const metadata = await (await fetch(url)).json();
  assert.deepEqual(metadata["dist-tags"], body["dist-tags"]);
  const dist = metadata.versions[record.version].dist;
  assert.equal(dist.integrity, record.integrity);
  assert.equal(dist.shasum, record.shasum);
  assert.equal(new URL(dist.tarball).origin, registry.url);
  assert.deepEqual(Buffer.from(await (await fetch(dist.tarball)).arrayBuffer()), bytes);
  assert.equal((await fetch(`${registry.url}/unknown-package`)).status, 404);
  assert.equal((await fetch(url, { method: "DELETE" })).status, 405);
});
