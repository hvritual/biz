# Enterprise Center Tenant Plan Change Lifecycle

## Status

EC-RI-06 extends the Enterprise Center from authoritative subscription/entitlement/usage reads into the tenant subscription-change lifecycle.

This document records the authority boundary implemented by `feat/ec-ri-06-tenant-change-preview`.

## Lifecycle

1. **Current authority read**
   - Current tenant comes from the authenticated principal.
   - Current subscription is read from `GET /v1/tenant/subscription`.
   - Entitlement limits and usage remain separate server authorities.

2. **Change target discovery**
   - Tenant calls `GET /v1/tenant/subscription/change-targets`.
   - Request contains no `tenant_id` or `sales_scope`.
   - SubscriptionChanges reads the current authoritative subscription and supplies its `sales_scope` to a transport-private Plan child capability.
   - Plan discovery returns only the latest **published + eligible** version per plan code.
   - A draft latest version must not hide a prior published eligible version.
   - The current plan/version is omitted from SWITCH targets.

3. **Authoritative preview**
   - Tenant calls `POST /v1/tenant/subscription/change-previews`.
   - Tenant scope comes only from trusted identity.
   - `SWITCH` accepts a target code/version and the server revalidates it against current subscription sales scope.
   - `RENEW` derives the current plan/version server-side.
   - `STOP_RENEWAL` accepts no target and no scheduled effective time.
   - Preview reuses CE-09 plan, entitlement, module dependency, quota and provisioning calculations.
   - Preview is a short-lived decision artifact. It does **not** mutate subscription or entitlement authority and does not start provisioning.

4. **Commercial/payment authority boundary**
   - Plan `price_ref` remains an opaque external pricing-owner reference.
   - The repository currently has no payment/billing authority that can prove payment completion or calculate a monetary amount.
   - Therefore the UI and tenant API must never convert `price_ref` into an amount or treat a button click as payment proof.
   - A non-`STOP_RENEWAL` preview whose target has `price_ref` returns `pricing_basis=PLATFORM_MANUAL_APPROVAL_REQUIRED`.
   - Tenant confirmation fails closed with `SUBSCRIPTION_CHANGE_EXTERNAL_APPROVAL_REQUIRED`.
   - Such a change must be admitted through the existing platform commercial/payment approval boundary before CE-09/CE-10 execution may apply it.

5. **Tenant confirmation for no-price-reference changes**
   - Tenant calls `POST /v1/tenant/subscription/changes/{change_id}/confirm`.
   - Confirmation requires the preview hash and a stable idempotency request ID.
   - Confirmation re-reads and revalidates the current subscription, target plan, entitlement snapshot, catalog, quota facts and provisioning requirements.
   - Stale/expired/conflicting previews fail closed.

6. **Execution result**
   - `APPLIED`: immediate authority mutation completed atomically.
   - `SCHEDULED`: current rights stay unchanged; `pendingChangeId` and durable time fence reserve the future transition.
   - `PROVISIONING`: current effective rights remain unchanged; `pendingChangeId` and durable provisioning task track external readiness.
   - `STOP_RENEWAL`: only the renewal intent changes; the current entitlement period is not shortened.

7. **Receipt/readback**
   - Tenant can read its own preview and receipt through tenant-scoped routes.
   - Receipt/readback never accepts a tenant ID from the browser.
   - Enterprise Center re-reads current subscription/entitlement/usage after confirmation.

## Tenant HTTP contract

| Phase | Method | Route | Authority |
| --- | --- | --- | --- |
| Targets | GET | `/v1/tenant/subscription/change-targets` | trusted tenant + published eligible Plan authority |
| Preview | POST | `/v1/tenant/subscription/change-previews` | trusted tenant + CE-09 projection |
| Preview readback | GET | `/v1/tenant/subscription/change-previews/{change_id}` | trusted tenant + original preview actor |
| Confirm | POST | `/v1/tenant/subscription/changes/{change_id}/confirm` | trusted tenant + fresh authority revalidation |
| Receipt | GET | `/v1/tenant/subscription/changes/{change_id}` | trusted tenant receipt authority |

## Security invariants

- Tenant self-service request messages contain no `tenant_id`.
- Browser never submits `sales_scope`, resource usage, payment status, entitlement/source versions or effective entitlement facts.
- `tenant.subscription.manage` is a dedicated IAM permission and is included in the protected owner-role permission closure.
- Platform CE-09 routes and permissions remain unchanged.
- Plan tenant-change helpers are transport-private C9 child operations: no HTTP route and no Web Session authentication.
- Existing CE-03, CE-07, CE-09 and CE-13 architecture gates explicitly enforce the split.
- `server/**` remains untouched.

## Frontend evidence

The focused Playwright qualification captures these 1440×900 states:

- target selection
- authoritative preview
- external commercial/payment approval boundary
- APPLIED receipt
- SCHEDULED receipt
- PROVISIONING receipt

The screenshot workflow is `.github/workflows/ec-ri-06-plan-change-web.yml` and produces artifact `ec-ri-06-plan-change-screenshots`.
