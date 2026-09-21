# #180 Delivery Admission

This is an execution admission boundary, not the Data Policy implementation.

## Authority

- Issue: #180.
- Human decisions: `docs/enterprise-center/decisions.md`.
- Contract boundary: `docs/enterprise-center/contracts.md`.
- Delivery authority remains the repository Delivery Execution Control Plane.

## Current disposition

The admission gate requires both decisions below to be `ACCEPTED`:

- `Q-009`: member/business-scope binding contract.
- `Q-011`: multiple Data Policy composition/conflict semantics.

At the current main baseline both are `PENDING_HUMAN`. Therefore an #180 candidate must produce a machine-readable `BLOCKED` receipt and must not enter Full Gate qualification.

The receipt explicitly records `semantics_implemented=false`. The checker is forbidden from inventing rules such as department-manager inheritance, automatic device visibility, policy union/intersection, or deny/allow precedence.

## Routing

An #180 candidate is selected when changed files include the mandatory
`integration/enterprise_180_*` qualification evidence. The integration evidence
is part of the candidate definition, so a business implementation cannot opt out
of admission merely by changing PR prose. Issue linkage remains recorded by the
normal Delivery Execution receipt and does not replace this route signal.

The existing canonical `enterprise-role-qualification.yml` is reused. No issue-specific workflow entrypoint and no 43rd Full Gate unit are added.

## Evidence

For an #180 PR, PR Qualification writes and uploads `enterprise180-admission.json`. While blocked:

- the admission step exits non-zero after the receipt is written;
- downstream candidate qualification cannot be treated as successful;
- PR Merge Gate cannot obtain the matching successful PR Qualification, so the fixed 42-unit Full Gate does not receive permission to advance.

When Q-009 and Q-011 are later accepted, the same gate becomes `ADMITTED`; that only permits implementation qualification to continue. It does not itself prove any Data Policy semantics.
