// Permit: assignee can Get their Ticket
permit(principal, action, resource)
when {
  action in ticketing-${project}-${stack}::ActionGroup::"Get" &&
  resource is ticketing-${project}-${stack}::Resource::"Ticket" &&
  resource.assignee == principal
};
