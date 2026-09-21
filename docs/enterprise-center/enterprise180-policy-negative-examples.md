# Enterprise #180 Policy Negative Examples

These cases are part of the accepted Q-009/Q-011 contract. They define denial/non-expansion behavior only; they do not implement Data Policy runtime behavior.

## N-01 Cross-tenant object injection

Given member M belongs to Tenant B and object/site A-1 belongs to Tenant A.

When a UI request or direct API call attempts to assign A-1 to M.

Then the candidate is not assignable and the write is rejected. No scope, policy reference, audit-success receipt, or derived projection may make A-1 visible to M.

## N-02 Unassignable object direct API injection

Given object B-2 belongs to the current tenant but is disabled, retired, or otherwise not returned by the authoritative assignable-object directory.

When a caller bypasses the UI and submits B-2 directly.

Then the API rejects the write with no partial mutation. UI disablement is not the authority; the same server-side assignability invariant is.

## N-03 Invalid policy reference

Given a Data Policy is expired, revoked, unknown, from another tenant, or its version no longer matches the expected CAS version.

When a role attempts to reference it.

Then the write is rejected. The policy version is a resource version only and cannot be treated as a global authorization version.

## N-04 Department movement cannot expand scope

Given a member has explicit scope {Site-1}.

When the member is moved into another department, becomes a department manager, or the department hierarchy is reorganized.

Then effective business-data scope remains bounded by the explicit scope and current policy. Department facts alone create no new business-data access.

## N-05 Broader second role cannot bypass member scope

Given Role-A and Role-B both grant an action, their current policy scopes are broader than the member explicit scope, and the member explicit scope is {Site-2}.

When Role-B is added.

Then action eligibility may come from the current Grant set, but effective data scope remains the intersection of applicable role policy scope, member explicit scope, and current tenant assignable scope. Site access outside {Site-2} is denied.

## N-06 Policy contraction is next-request effective

Given an old session previously accessed Site-3 and the current Data Policy is changed so Site-3 is outside the allowed scope.

When the same session sends the next sensitive business request for Site-3 after the policy write commits.

Then request-time authorization re-evaluates current facts and denies the request. No stale session claim, derived scope, or cached allow index may preserve the old access.

## Acceptance boundary

These examples must later be implemented as real MySQL/API/E2E negatives by the #180 business implementation candidate. The present decision-freeze task only fixes the semantics and Admission evidence.
