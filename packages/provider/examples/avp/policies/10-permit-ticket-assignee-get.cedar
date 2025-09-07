// Permit: assignee can get their ticket
permit(principal, getTicket, resource)
when { resource.assignee == principal };
