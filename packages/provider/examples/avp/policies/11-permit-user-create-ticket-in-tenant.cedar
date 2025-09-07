// Permit: a user can create tickets within their tenant
permit(principal, createTicket, resource)
when { resource.tenantId == principal.tenantId };
