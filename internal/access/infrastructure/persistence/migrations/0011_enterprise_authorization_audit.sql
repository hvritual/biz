ALTER TABLE biz_audit_events
  ADD COLUMN trace_id VARCHAR(128) NOT NULL DEFAULT '' AFTER request_id,
  ADD COLUMN resource_tenant_id VARCHAR(64) NOT NULL DEFAULT '' AFTER target,
  ADD COLUMN decision_reason VARCHAR(64) NOT NULL DEFAULT '' AFTER resource_tenant_id;

CREATE INDEX idx_audit_trace_id ON biz_audit_events(trace_id);
CREATE INDEX idx_audit_resource_tenant ON biz_audit_events(resource_tenant_id);
