// Consolidated guardrails

// Deny Global* actions when principal has tenantId
forbid(principal, action, resource)
when {
  principal has tenantId && (
    action in ActionGroup::"GlobalBatchCreate" ||
    action in ActionGroup::"GlobalCreate" ||
    action in ActionGroup::"GlobalBatchDelete" ||
    action in ActionGroup::"GlobalDelete" ||
    action in ActionGroup::"GlobalFind" ||
    action in ActionGroup::"GlobalGet" ||
    action in ActionGroup::"GlobalBatchUpdate" ||
    action in ActionGroup::"GlobalUpdate"
  )
};

// Deny tenant-scoped actions on resources missing tenantId
forbid(principal, action, resource)
when {
  !(resource has tenantId) && (
    action in ActionGroup::"BatchCreate" ||
    action in ActionGroup::"Create" ||
    action in ActionGroup::"BatchDelete" ||
    action in ActionGroup::"Delete" ||
    action in ActionGroup::"Find" ||
    action in ActionGroup::"Get" ||
    action in ActionGroup::"BatchUpdate" ||
    action in ActionGroup::"Update"
  )
};

// Deny actions not using approved action groups
forbid(principal, action, resource)
unless {
  action in ActionGroup::"BatchCreate" ||
  action in ActionGroup::"Create" ||
  action in ActionGroup::"BatchDelete" ||
  action in ActionGroup::"Delete" ||
  action in ActionGroup::"Find" ||
  action in ActionGroup::"Get" ||
  action in ActionGroup::"BatchUpdate" ||
  action in ActionGroup::"Update" ||
  action in ActionGroup::"GlobalBatchCreate" ||
  action in ActionGroup::"GlobalCreate" ||
  action in ActionGroup::"GlobalBatchDelete" ||
  action in ActionGroup::"GlobalDelete" ||
  action in ActionGroup::"GlobalFind" ||
  action in ActionGroup::"GlobalGet" ||
  action in ActionGroup::"GlobalBatchUpdate" ||
  action in ActionGroup::"GlobalUpdate"
};
