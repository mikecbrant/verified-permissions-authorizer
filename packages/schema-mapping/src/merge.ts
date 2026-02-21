import yaml from "yaml";

type Json = Record<string, any>;

const singleNs = (schema: Json): { ns: string; body: any } => {
  const keys = Object.keys(schema);
  if (keys.length !== 1)
    throw new Error(
      `schema must contain exactly one namespace, found ${keys.length}`,
    );
  const ns = keys[0];
  return { ns, body: (schema as any)[ns] };
};

const deepClone = <T>(v: T): T => JSON.parse(JSON.stringify(v));

const principals = new Set([
  "Tenant",
  "User",
  "Role",
  "GlobalRole",
  "TenantGrant",
]);

// Superset-only keys we allow adding to existing base defs
const ET_SUPERSET_KEYS = new Set(["resourceEntities"]);
const ACT_SUPERSET_KEYS = new Set(["entityMap", "input"]);

type MappingConfig = {
  // Minimal, top-level action identifier extraction to select per-action mappings.
  actions?: {
    appsync?: { path: string };
    apiGateway?: { path: string };
  };
};

type MergeResult = {
  // Full superset JSON including custom fields (resourceEntities, action.input, action.entityMap, optional root mappings)
  supersetJson: string;
  // Pruned Cedar JSON (safe to send to AVP PutSchema)
  cedarJson: string;
  namespace: string;
  mappings?: MappingConfig;
};

// pruneForCedar removes non-Cedar extensions from the superset in-place.
const pruneForCedar = (doc: Json): void => {
  const { ns, body } = singleNs(doc);
  const b = body as any;
  // Drop root-level superset sections if present
  if (b.mappings) delete b.mappings;
  // entityTypes: drop resourceEntities
  if (b.entityTypes && typeof b.entityTypes === "object") {
    for (const [, et] of Object.entries<any>(b.entityTypes)) {
      if (et && typeof et === "object" && et.resourceEntities)
        delete et.resourceEntities;
    }
  }
  // actions: drop input/entityMap
  if (b.actions && typeof b.actions === "object") {
    for (const [, act] of Object.entries<any>(b.actions)) {
      if (act && typeof act === "object") {
        if (act.input) delete act.input;
        if (act.entityMap) delete act.entityMap;
      }
    }
  }
  // Write back
  (doc as any)[ns] = b;
};

// mergeCedarSchemas merges a consumer partial superset into the base superset, enforcing:
// - exact single-namespace match
// - additions only for new entity types and actions
// - allowed augmentation of existing defs with superset-only keys
// - top-level `mappings.actions.*` retained (for actionId extraction)
// It returns both the merged superset JSON and a Cedar-safe JSON with extensions pruned.
const mergeCedarSchemas = (
  baseYaml: string,
  partialYaml: string,
): MergeResult => {
  const base = yaml.parse(baseYaml) as Json;
  const partial = yaml.parse(partialYaml) as Json;
  const { ns: bns, body: bbody } = singleNs(base);
  const { ns: pns, body: pbody } = singleNs(partial);
  if (bns !== pns)
    throw new Error(`namespace mismatch: base=${bns} partial=${pns}`);

  const out: Json = { [bns]: deepClone(bbody) };
  const obody = out[bns];

  // entityTypes
  if (pbody?.entityTypes) {
    obody.entityTypes ??= {};
    for (const [name, pdef] of Object.entries<any>(pbody.entityTypes)) {
      const exists = !!obody.entityTypes[name];
      if (!exists) {
        if (principals.has(name))
          throw new Error(`cannot add or modify principal type ${name}`);
        obody.entityTypes[name] = deepClone(pdef);
        continue;
      }
      // Augment existing base entity with superset-only keys
      const bdef = obody.entityTypes[name];
      if (typeof pdef !== "object" || pdef == null)
        throw new Error(`entityTypes.${name} must be an object`);
      for (const [k, v] of Object.entries<any>(pdef)) {
        if (!ET_SUPERSET_KEYS.has(k)) {
          // Disallow overriding Cedar fields like shape/memberOfTypes
          throw new Error(`cannot override base entityType ${name}.${k}`);
        }
        // Merge nested resourceEntities maps without overriding templates
        bdef.resourceEntities ??= {};
        for (const [tpl, tdef] of Object.entries<any>(v ?? {})) {
          if (bdef.resourceEntities[tpl])
            throw new Error(
              `cannot override existing resourceEntities template ${name}.${tpl}`,
            );
          bdef.resourceEntities[tpl] = deepClone(tdef);
        }
      }
    }
  }

  // actions
  if (pbody?.actions) {
    obody.actions ??= {};
    for (const [name, pdef] of Object.entries<any>(pbody.actions)) {
      const exists = !!obody.actions[name];
      if (!exists) {
        // New action: allow full Cedar fields + superset keys
        obody.actions[name] = deepClone(pdef);
        continue;
      }
      // Existing action: only allow superset-only keys; do not override Cedar fields
      const bdef = obody.actions[name];
      if (typeof pdef !== "object" || pdef == null)
        throw new Error(`actions.${name} must be an object`);
      for (const [k, v] of Object.entries<any>(pdef)) {
        if (!ACT_SUPERSET_KEYS.has(k)) {
          throw new Error(`cannot override base action ${name}.${k}`);
        }
        if (k === "entityMap") {
          bdef.entityMap ??= {};
          for (const [resType, tpl] of Object.entries<any>(v ?? {})) {
            if (bdef.entityMap[resType])
              throw new Error(
                `cannot override existing actions.${name}.entityMap for ${resType}`,
              );
            bdef.entityMap[resType] = tpl;
          }
        } else if (k === "input") {
          // Shallow replace/merge input by integration
          bdef.input ??= {};
          for (const [integration, idef] of Object.entries<any>(v ?? {})) {
            bdef.input[integration] = deepClone(idef);
          }
        }
      }
    }
  }

  // Optional: root-level mappings retained for actionId extraction only
  const mappings: MappingConfig | undefined = pbody?.mappings
    ? deepClone(pbody.mappings)
    : undefined;

  // Prepare outputs
  const supersetJson = JSON.stringify(out);
  const cedarDoc = deepClone(out);
  pruneForCedar(cedarDoc);
  const cedarJson = JSON.stringify(cedarDoc);
  return { supersetJson, cedarJson, namespace: bns, mappings };
};

// exports moved to end

// ---- Validation helpers and validator ----
const listTemplateVars = (s: unknown): string[] => {
  if (typeof s !== "string") return [];
  const vars = new Set<string>();
  const re = /\$([a-zA-Z0-9_]+)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(s)) != null) vars.add(m[1]);
  return [...vars];
};

const urlTemplateVars = (s: unknown): string[] => {
  if (typeof s !== "string") return [];
  const vars = new Set<string>();
  const re = /:([a-zA-Z0-9_]+)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(s)) != null) vars.add(m[1]);
  return [...vars];
};

// validateSuperset enforces VP-20 merge/validation rules. Returns an array of error messages (empty when valid).
const validateSuperset = (doc: Json): string[] => {
  const errors: string[] = [];
  let body: any;
  try {
    body = singleNs(doc).body;
  } catch {
    return ["schema must contain exactly one namespace"];
  }
  const entityTypes: Record<string, any> = body?.entityTypes ?? {};
  const actions: Record<string, any> = body?.actions ?? {};
  for (const [aname, adef] of Object.entries<any>(actions)) {
    const rtypes: string[] | undefined = adef?.appliesTo?.resourceTypes;
    // Skip canonical action groups (they typically specify principalTypes only)
    if (!Array.isArray(rtypes) || rtypes.length === 0) continue;
    const emap: Record<string, string> | undefined = adef?.entityMap;
    if (!emap) {
      errors.push(`actions.${aname}.entityMap is required`);
      continue;
    }
    for (const rt of rtypes) {
      const tplName = emap[rt];
      if (!tplName) {
        errors.push(
          `actions.${aname}.entityMap missing key for resourceType ${rt}`,
        );
        continue;
      }
      const tpl = entityTypes?.[rt]?.resourceEntities?.[tplName];
      if (!tpl) {
        errors.push(
          `actions.${aname}.entityMap.${rt} references missing template ${rt}.resourceEntities.${tplName}`,
        );
      } else {
        // Variable coverage per integration
        const needed = new Set([
          ...listTemplateVars(tpl.id),
          ...Object.values(tpl.attributes ?? {}).flatMap((v: any) =>
            listTemplateVars(v),
          ),
        ]);
        const appsyncVars = new Set(
          Object.keys(adef?.input?.appsync?.body ?? {}),
        );
        const restUrlVars = new Set(urlTemplateVars(adef?.input?.rest?.url));
        const restBodyVars = new Set(
          Object.keys(adef?.input?.rest?.body ?? {}),
        );
        const restQueryVars = new Set(
          typeof adef?.input?.rest?.query === "string"
            ? [adef.input.rest.query]
            : Object.keys(adef?.input?.rest?.query ?? {}),
        );
        // For AppSync: all needed vars must appear in appsync body mapping
        for (const v of needed) {
          if (!appsyncVars.has(v)) {
            errors.push(
              `actions.${aname} (appsync): template requires variable $${v} not provided in input.appsync.body`,
            );
          }
        }
        // For REST: allow vars from URL, body, or query
        for (const v of needed) {
          if (
            !(restUrlVars.has(v) || restBodyVars.has(v) || restQueryVars.has(v))
          ) {
            errors.push(
              `actions.${aname} (rest): template requires variable $${v} not provided in input.rest (url/body/query)`,
            );
          }
        }
      }
    }
  }
  return errors;
};

export { mergeCedarSchemas, pruneForCedar, validateSuperset };
export type { MappingConfig, MergeResult };
