// Offline npm wire-protocol fixture, not a general-purpose Registry.
// A Worker keeps HTTP requests running while the test invokes npm synchronously.
import crypto from "node:crypto";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import { isMainThread, parentPort, Worker, workerData } from "node:worker_threads";

export function startRegistry(packages, tarballsDir) {
  const worker = new Worker(new URL(import.meta.url), {
    workerData: { packages, tarballsDir },
  });
  return new Promise((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) => reject(new Error(`Registry fixture exited ${code}`)));
    worker.once("message", (address) => resolve({
      ...address,
      close: () => worker.terminate(),
    }));
  });
}

function json(response, status, value) {
  response.writeHead(status, {
    "Content-Type": "application/json",
    "Cache-Control": "no-store",
  });
  response.end(JSON.stringify(value));
}

function readJSON(request, limit) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let size = 0;
    let exceeded = false;
    request.on("data", (chunk) => {
      if (exceeded) return;
      size += chunk.length;
      if (size > limit) {
        exceeded = true;
        chunks.length = 0;
        reject(new Error("publish body too large"));
        return;
      }
      chunks.push(chunk);
    });
    request.on("error", reject);
    request.on("end", () => {
      if (exceeded) return;
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString("utf8")));
      } catch (error) {
        reject(error);
      }
    });
  });
}

if (!isMainThread) {
  const token = crypto.randomBytes(32).toString("hex");
  const records = new Map(workerData.packages.map((record) => {
    if (path.basename(record.tarball) !== record.tarball) {
      throw new Error("fixture tarball must be a basename");
    }
    const bytes = fs.readFileSync(path.join(workerData.tarballsDir, record.tarball));
    const integrity = `sha512-${crypto.createHash("sha512").update(bytes).digest("base64")}`;
    const shasum = crypto.createHash("sha1").update(bytes).digest("hex");
    if (record.integrity !== integrity || record.shasum !== shasum) {
      throw new Error(`fixture tarball integrity mismatch for ${record.name}`);
    }
    return [record.name, { ...record, bytes }];
  }));
  const published = new Map();
  let origin;
  const server = http.createServer(async (request, response) => {
    try {
      const pathname = decodeURIComponent(new URL(request.url, origin).pathname);
      if (request.method === "GET" && pathname === "/-/ping") {
        json(response, 200, {});
        return;
      }
      if (request.method === "POST" && pathname === "/-/npm/v1/security/advisories/bulk") {
        // All packages come from the reviewed manifest; never proxy an audit.
        request.resume();
        json(response, 200, {});
        return;
      }
      for (const record of published.values()) {
        if (request.method === "GET" && pathname === `/${record.name}/-/${record.tarball}`) {
          response.writeHead(200, { "Content-Type": "application/octet-stream" });
          response.end(record.bytes);
          return;
        }
      }
      const name = pathname.slice(1);
      const record = records.get(name);
      if (!record) {
        request.resume();
        json(response, 404, { error: "package is outside the fixture manifest" });
        return;
      }
      if (request.method === "GET") {
        const saved = published.get(name);
        json(response, saved ? 200 : 404, saved?.metadata ?? { error: "not published" });
        return;
      }
      if (request.method !== "PUT") {
        request.resume();
        json(response, 405, { error: "unsupported fixture operation" });
        return;
      }
      if (request.headers.authorization !== `Bearer ${token}`) {
        request.resume();
        json(response, 401, { error: "fixture token required" });
        return;
      }
      if (published.has(name)) {
        request.resume();
        json(response, 409, { error: "version already published" });
        return;
      }
      const body = await readJSON(request, Math.ceil(record.bytes.length * 4 / 3) + 1024 * 1024);
      if (published.has(name)) {
        json(response, 409, { error: "version already published" });
        return;
      }
      const version = body.versions?.[record.version];
      const attachments = Object.values(body._attachments ?? {});
      if (
        body.name !== name || version?.name !== name || version.version !== record.version ||
        Object.keys(body.versions).length !== 1 ||
        body["dist-tags"]?.[record.publishTag] !== record.version ||
        version.dist?.integrity !== record.integrity || version.dist?.shasum !== record.shasum ||
        attachments.length !== 1 || typeof attachments[0].data !== "string" ||
        !Buffer.from(attachments[0].data, "base64").equals(record.bytes)
      ) {
        json(response, 400, { error: "publication does not match the fixture manifest" });
        return;
      }
      version.dist.tarball = `${origin}/${record.name}/-/${record.tarball}`;
      published.set(name, { ...record, metadata: {
        name,
        "dist-tags": { [record.publishTag]: record.version },
        versions: { [record.version]: version },
      } });
      json(response, 201, { ok: "published" });
    } catch {
      json(response, 400, { error: "invalid fixture request" });
    }
  });
  server.listen(0, "127.0.0.1", () => {
    origin = `http://127.0.0.1:${server.address().port}`;
    parentPort.postMessage({ url: origin, token });
  });
}
