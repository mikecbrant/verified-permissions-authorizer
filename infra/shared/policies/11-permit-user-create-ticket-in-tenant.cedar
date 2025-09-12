// Permit: a user can Create tickets within their tenant
permit(principal, action, resource)
when {
  action in ticketing-${project}-${stack}::ActionGroup::"Create" &&
  resource is ticketing-${project}-${stack}::Resource::"Ticket" &&
  // tenant-scoped: principal (or grant) tenantId must match resource
  principal has tenantId && resource has tenantId && principal.tenantId == resource.tenantId
};
