type Json = Record<string, any>

type SupersetDoc = Json

const getNamespaceBody = (doc: SupersetDoc): any =>
  (doc as any)[Object.keys(doc)[0]]

const getByPath = (obj: unknown, path: string): unknown => {
  if (!obj || !path) return undefined
  return path
    .split('.')
    .reduce(
      (acc: any, k) => (acc && typeof acc === 'object' ? acc[k] : undefined),
      obj,
    )
}

const getFromJsonBody = (body: unknown, spec: string | undefined): unknown => {
  if (!spec) return undefined
  if (spec.startsWith('$.')) return getByPath(body, spec.slice(2))
  return body?.[spec]
}

const parseUrlTemplate = (
  template: string,
  path: string,
): Record<string, string> | undefined => {
  if (!template || !path) return undefined
  const tplSegs = template.split('/').filter(Boolean)
  const pathSegs = path.split('/').filter(Boolean)
  if (tplSegs.length !== pathSegs.length) return undefined
  const out: Record<string, string> = {}
  for (let i = 0; i < tplSegs.length; i++) {
    const t = tplSegs[i]
    const s = pathSegs[i]
    if (t.startsWith(':')) out[t.slice(1)] = decodeURIComponent(s)
    else if (t !== s) return undefined
  }
  return out
}

const substitute = (template: string, vars: Record<string, any>): string =>
  String(template).replace(/\$([a-zA-Z0-9_]+)/g, (_m, g1) =>
    (vars[g1] ?? '').toString(),
  )

type ExtractResult = {
  action?: string
  resource?: { entityType: string; entityId: string }
  vars: Record<string, any>
}

export type { ExtractResult, SupersetDoc }
export {
  getByPath,
  getFromJsonBody,
  getNamespaceBody,
  parseUrlTemplate,
  substitute,
}
