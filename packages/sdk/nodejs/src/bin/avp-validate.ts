#!/usr/bin/env node
/**
* AVP assets validator (SDK CLI)
*
* Validates a Cedar JSON schema (YAML/JSON), checks action-group conventions
* (PascalCase + Global* variants), scans policies for basic syntax, and
* validates canary config structure. No AWS calls are made.
*
* Usage:
*   avp-validate \
*     --schema ./path/to/schema.(yaml|yml|json) \
*     --policyDir ./path/to/policies \
*     [--canary ./path/to/canaries.yaml] \
*     [--mode off|warn|error]            # default: error
*/
import { readdirSync, readFileSync, statSync } from "node:fs";
import { extname, isAbsolute, join, resolve } from "node:path";
import process from "node:process";

import yaml from "yaml";

type Mode = "off" | "warn" | "error";
// allow optional CJS require for cedar-wasm without adding a dep
declare const require: any;

const args = new Map<string, string>();
for (let i = 2; i < process.argv.length; i++) {
  const a = process.argv[i];
  const m = a.match(/^--([^=]+)=(.*)$/);
  if (m) args.set(m[1], m[2]);
  else if (a.startsWith("--")) args.set(a.slice(2), "true");
}

const schemaArg = args.get("schema");
const policyDirArg = args.get("policyDir");
const canaryArg = args.get("canary");
const mode = ((args.get("mode") as Mode) ?? "error") as Mode;

if (!schemaArg) fail("--schema is required");
if (!policyDirArg) fail("--policyDir is required");

const schemaPath = abs(schemaArg);
const policyDir = abs(policyDirArg);
const canaryPath = canaryArg ? abs(canaryArg) : undefined;

// 1) Load and normalize schema to JS object
const { namespace, body, cedarJSON } = loadSchema(schemaPath);
// Enforce 100 KB size cap
const size = Buffer.byteLength(cedarJSON, "utf8");
if (size > 100_000) fail(`Schema JSON exceeds 100 KB (${size} bytes)`);
if (size >= 95_000)
  warn(
    `Schema JSON is ${size} bytes (>=95% of 100 KB limit). Consider simplifying entity shapes or splitting.`,
  );

// 2) Required principals/resources (provider-level)
const et = (body as any)?.entityTypes ?? {};
const requiredPrincipals = [
  "Tenant",
  "User",
  "Role",
  "GlobalRole",
  "TenantGrant",
];
const missingPrincipals = requiredPrincipals.filter((k) => !(k in et));
if (missingPrincipals.length)
  fail(`Missing required principals: ${missingPrincipals.join(", ")}`);

// 3) Action-group enforcement
const actions = Object.keys(((body as any)?.actions as any) ?? {});
const bad = enforceActionGroups(actions, mode);
if (bad.length) note(`Non-conforming actions: ${bad.join(", ")}`);

// 4) Policy syntax validation (best-effort)
const policyFiles = listPolicies(policyDir);
const syntaxErrors = validatePolicies(policyFiles);
if (syntaxErrors.length) {
  const txt = syntaxErrors.map((e) => `- ${e}`).join("\n");
  if (mode === "error") fail(`Policy syntax issues:\n${txt}`);
  warn(`Policy syntax issues (mode=warn):\n${txt}`);
}

// 5) Canary structure checks (optional)
if (canaryPath) validateCanaries(canaryPath);

ok(
  `OK: ${namespace} with ${Object.keys(et).length} entity types, ${actions.length} actions, ${policyFiles.length} policies`,
);

// ---- helpers ----

const abs = (p: string): string => (isAbsolute(p) ? p : resolve(process.cwd(), p));

const loadSchema = (path: string): {
  namespace: string;
  body: unknown;
  cedarJSON: string;
} => {
  const raw = readFileSync(path, "utf8");
  const ext = extname(path).toLowerCase();
  const doc = ext === ".json" ? JSON.parse(raw) : yaml.parse(raw);
  if (!doc || typeof doc !== "object" || Array.isArray(doc))
    fail("Schema must be an object mapping namespace -> body");
  const namespaces = Object.keys(doc as any);
  if (namespaces.length !== 1)
    fail(`Single namespace required; found ${namespaces.length}`);
  const ns = namespaces[0];
  const body = (doc as any)[ns];
  const cedarJSON = JSON.stringify(doc);
  return { namespace: ns, body, cedarJSON };
};

const CANONICAL = new Set([
  "BatchCreate",
  "Create",
  "BatchDelete",
  "Delete",
  "Find",
  "Get",
  "BatchUpdate",
  "Update",
  // Global*
  "GlobalBatchCreate",
  "GlobalCreate",
  "GlobalBatchDelete",
  "GlobalDelete",
  "GlobalFind",
  "GlobalGet",
  "GlobalBatchUpdate",
  "GlobalUpdate",
]);

const enforceActionGroups = (actions: string[], mode: Mode): string[] => {
  if (mode === "off") return [];
  const bad: string[] = [];
  for (const a of actions) {
    let ok = false;
    for (const g of CANONICAL) {
      if (a.startsWith(g)) {
        ok = true;
        break;
      }
    }
    if (!ok) bad.push(a);
  }
  if (bad.length && mode === "error")
    fail(
      `Actions not aligned to canonical action groups (case-sensitive) ${Array.from(
        CANONICAL,
      ).join(", ")}: ${bad.join(", ")}`,
    );
  if (bad.length)
    warn(`Actions not aligned to canonical groups (case-sensitive): ${bad.join(", ")}`);
  return bad;
};

// no helper needed beyond prefix matching

const listPolicies = (dir: string): string[] => {
  const st = statSync(dir);
  if (!st.isDirectory()) fail(`policyDir is not a directory: ${dir}`);
  const res: string[] = [];
  const entries = readdirSync(dir, { withFileTypes: true });
  for (const e of entries) {
    const p = join(dir, e.name);
    if (e.isDirectory()) res.push(...listPolicies(p));
    else if (e.isFile() && p.endsWith(".cedar")) res.push(p);
  }
  return res.sort();
};

const validatePolicies = (files: string[]): string[] => {
  const errs: string[] = [];
  let cedar: any | undefined;
  try {
    // Optional; not required to run
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    cedar = require("@cedar-policy/cedar-wasm/nodejs");
  } catch {}
  for (const f of files) {
    const text = readFileSync(f, "utf8");
    if (cedar && typeof cedar.parsePolicySet === "function") {
      try {
        cedar.parsePolicySet(text);
      } catch (e: any) {
        errs.push(`${f}: ${e?.message ?? String(e)}`);
      }
    } else if (!/\b(permit|forbid)\s*\(\s*principal/i.test(text)) {
      errs.push(`${f}: does not appear to contain a Cedar policy statement`);
    }
  }
  return errs;
};

const validateCanaries = (path: string): void => {
  const doc = yaml.parse(readFileSync(path, "utf8"));
  const cases: any[] = doc?.cases ?? [];
  if (!Array.isArray(cases)) fail(`Invalid canary file (cases must be an array): ${path}`);
  // Structural checks only; execution happens in the provider
  const requiredKeys = ["principal", "action", "resource", "expect"];
  for (let i = 0; i < cases.length; i++) {
    const c = cases[i];
    for (const k of requiredKeys) if (!(k in c)) fail(`canary #${i + 1} missing ${k}`);
  }
};

const fail = (msg: string): never => {
  console.error(`[avp-validate] ${msg}`);
  process.exit(2);
};
const warn = (msg: string): void => {
  console.warn(`[avp-validate] warn: ${msg}`);
};
const note = (msg: string): void => {
  console.log(`[avp-validate] ${msg}`);
};
const ok = (msg: string): never => {
  console.log(`[avp-validate] ${msg}`);
  process.exit(0);
};
