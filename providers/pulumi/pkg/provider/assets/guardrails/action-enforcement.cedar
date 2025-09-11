// Guardrail: enforce use of canonical action groups only
forbid(principal, action, resource)
unless {
    action in ${NAMESPACE}::ActionGroup::"BatchCreate" ||
    action in ${NAMESPACE}::ActionGroup::"Create" ||
    action in ${NAMESPACE}::ActionGroup::"BatchDelete" ||
    action in ${NAMESPACE}::ActionGroup::"Delete" ||
    action in ${NAMESPACE}::ActionGroup::"Find" ||
    action in ${NAMESPACE}::ActionGroup::"Get" ||
    action in ${NAMESPACE}::ActionGroup::"BatchUpdate" ||
    action in ${NAMESPACE}::ActionGroup::"Update" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalBatchCreate" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalCreate" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalBatchDelete" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalDelete" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalFind" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalGet" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalBatchUpdate" ||
    action in ${NAMESPACE}::ActionGroup::"GlobalUpdate"
};
