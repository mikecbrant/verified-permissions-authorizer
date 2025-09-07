// Permit: a user can Create tickets within their tenant
permit(
  principal,
  action in Action::"Create",
  resource is Resource::"Ticket"
)
when { principal has tenantId && resource.tenantId == principal.tenantId };
when { resource.tenantId == principal.tenantId };
