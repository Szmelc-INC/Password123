// Cross-implementation check: payloads produced by the Go CLI (filippo.io/age)
// must decrypt with the exact bundle the browser portal ships (typage).
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { Decrypter } from "../../docs/assets/age.js";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const read = (p) => readFileSync(path.join(root, p));
const demoIdentity = read("demo/demo-identity.txt").toString().split("\n").find((l) => l.startsWith("AGE-SECRET-KEY-1"));
const demoManifest = read("demo/manifest.yaml").toString();
const field = (name) => [...demoManifest.matchAll(new RegExp(`${name}: "([0-9a-f]{64})"`, "g"))].map((m) => m[1]);

test("demo payload decrypts in the browser bundle and matches the CLI plaintext digest", async () => {
  const d = new Decrypter();
  d.addIdentity(demoIdentity);
  const ct = read("payloads/demo-welcome.age");
  assert.equal(createHash("sha256").update(ct).digest("hex"), field("sha256")[0], "ciphertext digest");
  const pt = await d.decrypt(new Uint8Array(ct));
  assert.equal(createHash("sha256").update(pt).digest("hex"), field("plaintext_sha256")[0], "plaintext digest");
  const doc = JSON.parse(new TextDecoder().decode(pt));
  assert.equal(doc.kind, "demo");
});

test("the demo identity cannot open any real challenge payload", async () => {
  const files = readdirSync(path.join(root, "payloads")).filter((f) => f.endsWith(".age") && !f.startsWith("demo-"));
  assert.ok(files.length > 0);
  for (const f of files) {
    const d = new Decrypter();
    d.addIdentity(demoIdentity);
    await assert.rejects(d.decrypt(new Uint8Array(read(`payloads/${f}`))), `${f} must not open with the demo identity`);
  }
});

test("a modified payload is rejected, not returned", async () => {
  const ct = new Uint8Array(read("payloads/demo-welcome.age"));
  ct[ct.length - 1] ^= 0x01;
  const d = new Decrypter();
  d.addIdentity(demoIdentity);
  await assert.rejects(d.decrypt(ct));
});
