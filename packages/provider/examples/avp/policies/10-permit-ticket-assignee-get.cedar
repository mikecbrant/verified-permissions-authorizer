// Permit: assignee can Get their Ticket
permit(principal,
  action in Action::"Get",
  resource is Resource::"Ticket"
)
when { principal == resource.assignee };
when { resource.assignee == principal };
