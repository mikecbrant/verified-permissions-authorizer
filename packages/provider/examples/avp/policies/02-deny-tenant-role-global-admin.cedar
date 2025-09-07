// Guardrail: deny when a tenant-scoped role attempts a global admin action
// This example assumes global admin actions are those with prefix "get" on global resources
forbid(principal, action, resource)
when {
  // if the principal is in a tenant-scoped role and action targets a global-only resource
  principal in Role &&
  action == getFile &&
  resource in Files &&
  resource.tenantId == "GLOBAL"
};
