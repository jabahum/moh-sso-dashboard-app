# Contextual authorization

This page defines how scoped operations are authorized against health context: the decision sequence, least-privilege rules, where to enforce checks, temporary access and denial logging. It is for developers implementing context-scoped features; the model is described in [Health context architecture](../architecture/health-context.md).

## Decision sequence

Every scoped operation must evaluate:

1. Authentication.
2. Functional permission.
3. Access to the owning portal system/client.
4. The built-in National baseline, plus direct and group-derived health-context assignments.
5. Assignment validity, node status, and scope mode.
6. Resource ownership or authorized query predicate.

Authenticated actors outside the resource scope receive `403`. Malformed
context identifiers receive `400`, as does a missing header on a contextual route
(`HEALTH_CONTEXT_REQUIRED`). If access cannot be verified, the response is `503`
(`HEALTH_CONTEXT_UNAVAILABLE`). Detailed database and hierarchy errors stay in
server logs.

Every authenticated user has the enabled Uganda National context and its
descendants by default. This policy grants geographic scope, not application
roles or functional permissions. The baseline is identified as `DEFAULT` in
effective-access responses and explanations. Disabling the National node stops
this baseline; unrelated hierarchy roots are not included.

## Least privilege

- Super-admin is not an implicit context bypass.
- Disabled nodes and expired assignments grant no assignment-based access.
  Expiring an explicit National assignment does not remove the built-in baseline.
- Group-derived permission and group-derived context are evaluated and
  explained separately.
- Frontend visibility never substitutes for backend enforcement.
- Exports and downloads use the same predicates as list and detail endpoints.
- Aggregates must not reveal restricted lower-level data.

## Resource checks

Use middleware only to validate a selected context when that is sufficient.
For contextual route groups, the middleware (`RequireAccessibleSelection` in
`backend/internal/features/health_context/handler.go`) fails closed when a user
with any effective context omits the context header. It allows a missing header
only when the user has no effective contexts. Because of the National baseline,
that happens only when the `UG` node is disabled or missing and the user has no
other assignments.
When ownership is known only after loading a resource, enforce access in the
feature service. Repositories should accept already-authorized context IDs and
include them in SQL predicates.

## Temporary and emergency access

Emergency or delegated access is not enabled by default. A future implementation
must require a reason, approval, explicit expiry, audit events, and administrator
notification.

The assignment model already supports `valid_from` and `valid_until`, so
time-bound access can be represented without adding a bypass. There is
currently no emergency override switch and no hidden super-admin exception.
Before enabling delegated or emergency workflows, add an approval record,
mandatory reason, approver separation, expiry enforcement, administrator
notification, and dedicated audit events. Expiry must be enforced by effective
context queries, not by a background cleanup job alone.

## Denial logging

Context middleware emits a structured warning log, `health context authorization
denied`, with the request ID, feature (the first path segment after `/api/v1/`)
and reason code. Logs do not include patient data, resource
payloads, user contact details, or context-specific metric labels.
