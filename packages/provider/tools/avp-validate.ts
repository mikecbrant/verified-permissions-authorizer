#!/usr/bin/env bun
/**
* Validate AVP schema/policy assets locally.
* Usage: bun run packages/provider/tools/avp-validate.ts --dir packages/provider/examples/avp [--mode warn|error]
*/
import { readFileSync, statSync } from "node:fs";
import { join, isAbsolute } from "node:path";
import process from "node:process";
import yaml from "yaml";

type Mode = "off" | "warn" | "error";
const ARGS = new Map<string, string>();
for (let i = 2; i < process.argv.length; i++) {
  const a = process.argv[i];
  const m = a.match(/^--([^=]+)=(.*)$/);
  if (m) ARGS.set(m[1], m[2]);
  else if (a.startsWith("--")) ARGS.set(a.slice(2), "true");
}
const dir = ARGS.get("dir") ?? ARGS.get("d");
if (!dir) {
  console.error("--dir is required (path to asset directory)");
  process.exit(2);
}
const mode = (ARGS.get("mode") as Mode) ?? "warn";
const base = isAbsolute(dir) ? dir : join(process.cwd(), dir);
try {
  const st = statSync(base);
  if (!st.isDirectory()) throw new Error();
} catch {
  console.error(`dir not found or not a directory: ${base}`);
  process.exit(2);
}

// Find schema file
const schemaFile = ["schema.yaml", "schema.yml", "schema.json"]
  .map((f) => join(base, f))
  .find((p) => {
    try {
      return statSync(p).isFile();
    } catch {
      return false;
    }
  });
if (!schemaFile) {
  console.error(
    `No schema file found in ${base} (looked for schema.yaml|schema.yml|schema.json)`,
  );
  process.exit(2);
}

const doc = (() => {
  const raw = readFileSync(schemaFile, "utf8");
  if (schemaFile.endsWith(".json")) return JSON.parse(raw);
  return yaml.parse(raw);
})();
if (!doc || typeof doc !== "object" || Array.isArray(doc)) {
  console.error("Schema must be an object mapping namespace to body");
  process.exit(2);
}
const namespaces = Object.keys(doc as any);
if (namespaces.length !== 1) {
  console.error(
    `AVP supports a single namespace per schema; found ${namespaces.length}`,
  );
  process.exit(2);
}
const ns = namespaces[0];
const body = (doc as any)[ns];
const et = body?.entityTypes;
if (!et || typeof et !== "object") {
  console.error("schema must define entityTypes");
  process.exit(2);
}
const requiredPrincipals = [
  "Tenant",
  "User",
  "Role",
  "GlobalRole",
  "TenantGrant",
];
const missing = requiredPrincipals.filter((k) => !(k in et));
if (missing.length) {
  console.error(`Missing required entity types: ${missing.join(", ")}`);
  process.exit(2);
}
// Action-group check
const actions = Object.keys(body?.actions ?? {});
const groups = new Set([
  "BatchCreate",
  "Create",
  "BatchDelete",
  "Delete",
  "Find",
  "Get",
  "BatchUpdate",
  "Update",
  "GlobalBatchCreate",
  "GlobalCreate",
  "GlobalBatchDelete",
  "GlobalDelete",
  "GlobalFind",
  "GlobalGet",
  "GlobalBatchUpdate",
  "GlobalUpdate",
]);
const matchesCanonical = (s: string): boolean => {
  const lower = s.toLowerCase();
  for (const g of groups) if (lower.startsWith(String(g).toLowerCase())) return true;
  return false;
};
const bad = actions.filter((a) => !matchesCanonical(a));
if (bad.length) {
  const msg = `Actions not aligned to canonical action groups: ${bad.join(", ")}`;
  if (mode === "error") {
    console.error(msg);
    process.exit(2);
  }
  console.warn(`[warn] ${msg}`);
}

console.log(
  `OK: ${ns} with ${Object.keys(et).length} entities and ${actions.length} actions`,
);

// (no helper needed beyond matchesCanonical)
