// Guardrail: deny when principal and resource belong to different tenants
forbid(principal, action, resource)
when { principal.tenantId != resource.tenantId };
