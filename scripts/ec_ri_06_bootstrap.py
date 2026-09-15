from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if new in text:
        return text
    if old not in text:
        raise SystemExit(f"{label} anchor not found")
    return text.replace(old, new, 1)


proto = Path("contracts/proto/commercial/v1/subscription.proto")
text = proto.read_text()
text = replace_once(
    text,
    "message GetTenantSubscriptionRequest { string tenant_id=1; }\n",
    "message GetTenantSubscriptionRequest { string tenant_id=1; }\nmessage GetMySubscriptionRequest {}\n",
    "subscription request",
)
platform_rpc = ' rpc GetTenantSubscription(GetTenantSubscriptionRequest) returns (TenantSubscriptionDTO) { option (google.api.http)={get:"/v1/platform/tenants/{tenant_id}/subscription"}; option (yunka.dsl.v1.operation)={id:"commercial.subscription.get" use_case:"get_tenant_subscription" permissions:"platform.subscription.read" permissions:"platform.tenant.read" permission_mode:PERMISSION_ALL tenant_required:false authentication:AUTHENTICATION_API_KEY authentication:AUTHENTICATION_WEB_SESSION execution:{transaction:TRANSACTION_READ_ONLY idempotency:IDEMPOTENCY_NONE}}; }\n'
tenant_rpc = ' rpc GetMySubscription(GetMySubscriptionRequest) returns (TenantSubscriptionDTO) { option (google.api.http)={get:"/v1/tenant/subscription"}; option (yunka.dsl.v1.operation)={id:"commercial.subscription.get_my" use_case:"get_my_subscription" permissions:"tenant.entitlement.read" permission_mode:PERMISSION_ALL tenant_required:true authentication:AUTHENTICATION_API_KEY authentication:AUTHENTICATION_WEB_SESSION execution:{transaction:TRANSACTION_READ_ONLY idempotency:IDEMPOTENCY_NONE}}; }\n'
if tenant_rpc not in text:
    if platform_rpc not in text:
        raise SystemExit("subscription rpc anchor not found")
    text = text.replace(platform_rpc, platform_rpc + tenant_rpc, 1)
proto.write_text(text)

service = Path("internal/commercial/application/subscriptionmanagement/internal/usecase/service.go")
text = service.read_text()
actor = '''func actor(ctx context.Context) (string, error) {
\tp, ok := identity.FromContext(ctx)
\tif !ok || !p.Authenticated || p.Subject == "" || p.TenantID != "" {
\t\treturn "", subscription.ErrScope
\t}
\treturn p.Subject, nil
}
'''
tenant_actor = '''func tenantActor(ctx context.Context) (identity.Principal, error) {
\tp, ok := identity.FromContext(ctx)
\tif !ok || !p.Authenticated || p.Subject == "" || p.TenantID == "" {
\t\treturn identity.Principal{}, subscription.ErrScope
\t}
\treturn p, nil
}
'''
if tenant_actor not in text:
    if actor not in text:
        raise SystemExit("subscription actor anchor not found")
    text = text.replace(actor, actor + tenant_actor, 1)
method = '''func (s *service) GetMySubscription(ctx context.Context, _ *v1.GetMySubscriptionRequest) (*v1.TenantSubscriptionDTO, error) {
\tp, e := tenantActor(ctx)
\tif e != nil {
\t\treturn nil, exposed(e)
\t}
\tv, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionRepositories]) (subscription.Subscription, error) {
\t\treturn sc.Repositories().Subscriptions.GetBase(sc.Context(), p.TenantID, false)
\t})
\tif e != nil {
\t\treturn nil, exposed(e)
\t}
\treturn dto(v), nil
}
'''
if method not in text:
    text = text.rstrip() + "\n\n" + method
service.write_text(text)

model = Path("internal/access/domain/model.go")
text = model.read_text()
owner_anchor = "var OwnerRequiredPermissions = []string{\n"
if '"tenant.entitlement.read"' not in text:
    if owner_anchor not in text:
        raise SystemExit("owner permission anchor not found")
    text = text.replace(
        owner_anchor,
        owner_anchor + '\t"commercial.catalog.read",\n\t"tenant.entitlement.read",\n',
        1,
    )
model.write_text(text)

mapping = Path("contracts/commercial/operation-capabilities.v1.json")
text = mapping.read_text()
platform_mapping = '''    {
      "operation_id": "commercial.subscription.get",
      "classification": "platform_management",
      "capability_codes": [],
      "children": [],
      "exemption_reason": "CE-08 platform default subscription control/bootstrap; never a tenant-purchased business operation and never bypasses IAM."
    },
'''
tenant_mapping = '''    {
      "operation_id": "commercial.subscription.get_my",
      "classification": "recovery",
      "capability_codes": [],
      "children": [],
      "exemption_reason": "EC-RI-06 authenticated current-tenant commercial projection; it exposes only the caller tenant subscription selected from trusted identity and remains protected by tenant.entitlement.read."
    },
'''
if tenant_mapping not in text:
    if platform_mapping not in text:
        raise SystemExit("commercial capability mapping anchor not found")
    text = text.replace(platform_mapping, platform_mapping + tenant_mapping, 1)
if '"mapping_version": "13"' in text:
    text = text.replace('"mapping_version": "13"', '"mapping_version": "14"', 1)
mapping.write_text(text)

router = Path("web/src/router/index.ts")
text = router.read_text()
old = "component: () => import('@/features/enterprise/pages/PlansView.vue'),"
new = "component: () => import('@/features/enterprise/pages/PlansEntryView.vue'),"
if new not in text:
    if old not in text:
        raise SystemExit("enterprise plan router anchor not found")
    text = text.replace(old, new, 1)
router.write_text(text)
