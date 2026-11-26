// Password123 portal. A reader, not an encryption console.
// Rules (ARCHITECTURE.md section 9): same-origin fetches only, identities in
// memory only, plaintext rendered with textContent, explicit lock.
import { Decrypter, armor } from "./assets/age.js";

const $ = (id) => document.getElementById(id);
const state = { site: null, lessons: [], current: null, identity: null };

function el(tag, attrs = {}, ...children) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === "class") n.className = v;
    else n.setAttribute(k, v);
  }
  for (const c of children) {
    if (c === null || c === undefined) continue;
    n.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return n;
}

function status(msg, kind = "") {
  const s = $("vault-status");
  s.className = "status " + kind;
  s.textContent = msg;
}

async function sha256Hex(bytes) {
  const d = await crypto.subtle.digest("SHA-256", bytes);
  return [...new Uint8Array(d)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

function renderList() {
  const mod = $("module-filter").value;
  const q = $("search").value.trim().toLowerCase();
  const ul = $("lesson-list");
  ul.replaceChildren();
  for (const l of state.lessons) {
    if (mod && l.module !== mod) continue;
    const hay = `${l.id} ${l.title} ${l.category} ${l.summary}`.toLowerCase();
    if (q && !hay.includes(q)) continue;
    const meta = `${l.status} · ${l.difficulty}${l.payload ? " · challenge" : ""}`;
    const b = el("button", { type: "button", "data-id": l.id }, el("span", { class: "lid" }, l.id), el("span", { class: "ltitle" }, l.title), el("span", { class: "lmeta" }, meta));
    if (state.current && state.current.id === l.id) b.setAttribute("aria-current", "true");
    b.addEventListener("click", () => select(l.id, true));
    ul.append(el("li", {}, b));
  }
}

function section(title, ...body) {
  return [el("h3", {}, title), ...body];
}

function renderLesson(l) {
  const art = $("lesson");
  const badges = el("div", { class: "badges" },
    el("span", { class: "badge" }, l.module),
    el("span", { class: "badge" }, l.difficulty),
    el("span", { class: "badge " + (l.status === "stable" ? "ok" : l.status === "deprecated" ? "bad" : "") }, l.status),
    el("span", { class: "badge " + (l.recommended_for_new_systems ? "ok" : "bad") }, l.recommended_for_new_systems ? "ok for new systems" : "not for new systems"),
    ...l.labels.map((x) => el("span", { class: "badge" }, x)));
  const parts = [el("h2", {}, l.title), el("p", { class: "mono muted" }, l.id), badges, el("p", { class: "summary" }, l.summary)];
  const g = l.guide;
  parts.push(...section("What it does", el("p", {}, g.what)));
  parts.push(...section("What problem it solves", el("p", {}, g.problem)));
  parts.push(...section("Inputs and outputs", el("p", {}, g.inputs_outputs)));
  parts.push(...section("Keys and key management", el("p", {}, g.key_management)));
  parts.push(...section("Common mistakes", el("ul", {}, ...g.common_mistakes.map((m) => el("li", {}, m)))));
  parts.push(...section("Use in new systems", el("p", {}, g.new_systems)));

  const results = state.site.vector_results.filter((r) => r.lesson_id === l.id);
  if (results.length) {
    const files = state.site.vector_files.filter((f) => l.vectors.includes(f.path));
    const rows = results.map((r) => {
      const v = files.flatMap((f) => f.vectors).find((x) => x.id === r.id);
      const src = v ? v.source.document + (v.source.test_case ? `, ${v.source.test_case}` : "") : "";
      const link = v ? el("a", { href: v.source.url, rel: "noopener noreferrer" }, src) : src;
      return el("tr", {}, el("td", { class: "mono" }, r.id), el("td", {}, r.kind), el("td", {}, el("span", { class: "badge " + (r.ok ? "ok" : "bad") }, r.ok ? "pass" : "fail")), el("td", {}, link));
    });
    parts.push(...section("Published test vectors (recomputed at build time)",
      el("div", { class: "table-wrap" }, el("table", { class: "vectors" },
        el("thead", {}, el("tr", {}, el("th", {}, "vector"), el("th", {}, "kind"), el("th", {}, "result"), el("th", {}, "source"))),
        el("tbody", {}, ...rows)))));
  }
  parts.push(...section("Reproduce locally", el("pre", { class: "plaintext" }, g.reproduce.trim())));

  if (l.challenge) {
    const kv = el("dl", { class: "kv" });
    for (const k of Object.keys(l.challenge.public_inputs).sort()) {
      const v = l.challenge.public_inputs[k];
      kv.append(el("dt", {}, k), el("dd", {}, typeof v === "string" ? v : JSON.stringify(v, null, 2)));
    }
    parts.push(...section("Challenge", el("p", {}, l.challenge.prompt), kv,
      l.challenge.submit ? el("p", { class: "muted" }, "Answer: " + l.challenge.submit) : null,
      l.payload ? el("p", { class: "muted" }, "Encrypted solution: ", el("code", {}, l.payload.path), " · sha256 ", el("code", {}, l.payload.sha256.slice(0, 16) + "…")) : null));
  }
  parts.push(...section("References", el("ul", {}, ...l.references.map((r) => el("li", {}, el("a", { href: r.url, rel: "noopener noreferrer" }, r.name))))));
  art.replaceChildren(...parts);
}

function payloadOptions() {
  const sel = $("payload-select");
  sel.replaceChildren();
  for (const p of state.site.demo.payloads) sel.append(el("option", { value: p.path, "data-sha": p.sha256 }, `${p.id} (demo)`));
  for (const l of state.lessons) if (l.payload) sel.append(el("option", { value: l.payload.path, "data-sha": l.payload.sha256 }, `${l.id} solution`));
}

function select(id, scroll) {
  const l = state.lessons.find((x) => x.id === id);
  if (!l) return;
  state.current = l;
  renderLesson(l);
  renderList();
  if (l.payload) $("payload-select").value = l.payload.path;
  history.replaceState(null, "", "#" + encodeURIComponent(l.id));
  if (scroll) $("lesson").scrollIntoView({ block: "start", behavior: "smooth" });
}

function takeIdentity() {
  const raw = $("identity").value.trim();
  if (raw) {
    const line = raw.split(/\r?\n/).map((s) => s.trim()).find((s) => s.startsWith("AGE-SECRET-KEY-"));
    if (!line) throw new Error("No AGE-SECRET-KEY line found. SSH keys are not supported in the browser; use the CLI.");
    state.identity = line;
    $("identity").value = "";
  }
  if (!state.identity) throw new Error("Provide an identity first.");
  return state.identity;
}

async function decrypt() {
  const out = $("plaintext");
  out.hidden = true;
  out.textContent = "";
  try {
    const id = takeIdentity();
    const opt = $("payload-select").selectedOptions[0];
    if (!opt) throw new Error("No payload selected.");
    status("Fetching ciphertext…");
    const res = await fetch(opt.value, { cache: "no-store", credentials: "omit" });
    if (!res.ok) throw new Error(`Could not fetch ${opt.value} (${res.status}).`);
    let bytes = new Uint8Array(await res.arrayBuffer());
    const digest = await sha256Hex(bytes);
    if (digest !== opt.dataset.sha) throw new Error("Ciphertext digest does not match the manifest. Refusing to decrypt.");
    const head = new TextDecoder().decode(bytes.slice(0, 40));
    if (head.startsWith("-----BEGIN AGE ENCRYPTED FILE-----")) bytes = armor.decode(new TextDecoder().decode(bytes));
    const d = new Decrypter();
    d.addIdentity(id);
    const pt = await d.decrypt(bytes, "text");
    let shown = pt;
    try { shown = JSON.stringify(JSON.parse(pt), null, 2); } catch { /* not JSON: show as text */ }
    out.textContent = shown;
    out.hidden = false;
    status("");
    $("vault-status").append(el("span", { class: "hi" }, "decrypted locally"), " Press Lock to clear it from the page.");
    $("vault-status").className = "status ok";
  } catch (e) {
    // Never log or echo identities or plaintext; keep messages generic.
    const msg = e && e.message && !/AGE-SECRET-KEY/.test(e.message) ? e.message : "Decryption failed.";
    status(/no identity matched|no identities matched/i.test(msg) ? "This identity is not a recipient of that payload." : msg, "err");
  }
}

function lock() {
  state.identity = null;
  $("identity").value = "";
  $("identity-file").value = "";
  const out = $("plaintext");
  out.textContent = "";
  out.hidden = true;
  status("Locked. Identity and plaintext were dropped from the page (JavaScript cannot guarantee memory is wiped).", "ok");
}

async function main() {
  const res = await fetch("generated/manifest.json", { cache: "no-store", credentials: "omit" });
  if (!res.ok) throw new Error("manifest not found; build the site with p123 pages build");
  state.site = await res.json();
  state.lessons = state.site.lessons;
  for (const m of state.site.modules) $("module-filter").append(el("option", { value: m }, m));
  const pass = state.site.vector_results.filter((r) => r.ok).length;
  $("build-info").textContent = `${state.lessons.length} lessons · ${pass}/${state.site.vector_results.length} vectors pass · revision ${state.site.source.revision.slice(0, 12)}`;
  payloadOptions();
  $("module-filter").addEventListener("change", renderList);
  $("search").addEventListener("input", renderList);
  $("decrypt").addEventListener("click", decrypt);
  $("lock").addEventListener("click", lock);
  $("identity").addEventListener("focus", () => $("identity").classList.add("revealed"));
  $("identity").addEventListener("blur", () => $("identity").classList.remove("revealed"));
  $("identity-file").addEventListener("change", async (ev) => {
    const f = ev.target.files && ev.target.files[0];
    if (!f) return;
    if (f.size > 64 * 1024) return status("That file is too large to be an age identity.", "err");
    $("identity").value = await f.text();
    ev.target.value = "";
    status("Identity file read into memory. Press Decrypt.");
  });
  $("use-demo").addEventListener("click", async () => {
    const r = await fetch(state.site.demo.identity, { cache: "no-store", credentials: "omit" });
    $("identity").value = await r.text();
    $("payload-select").value = state.site.demo.payloads[0].path;
    status("Demo identity loaded. It is public and opens demo payloads only.");
  });
  window.addEventListener("pagehide", lock);
  const want = decodeURIComponent(location.hash.slice(1));
  select(state.lessons.some((l) => l.id === want) ? want : state.lessons[0].id, false);
}

main().catch((e) => {
  $("lesson").replaceChildren(el("p", { class: "status err" }, "Could not load the site: " + e.message));
});
