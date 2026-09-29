import type { CommercialStateCode, CommercialStateKind } from '@/services/commercial/state-vocabulary.generated'

// Translation keys only. The allowed wire values are derived from Go/Proto.
export const commercialStateTerms = {
  "subscriptionState": {
    "ACTIVE": "active",
    "TRIAL": "trial",
    "GRACE": "grace",
    "RESTRICTED": "restricted",
    "ENDED": "ended"
  },
  "planState": {
    "DRAFT": "draft",
    "PUBLISHED": "published",
    "RETIRED": "retired"
  },
  "changeClassification": {
    "UPGRADE": "upgrade",
    "DOWNGRADE": "downgrade",
    "SAME_TIER": "sameTier",
    "RENEW": "renewal",
    "STOP_RENEWAL": "stopRenewal"
  },
  "effectiveMode": {
    "IMMEDIATE": "immediate",
    "SCHEDULED": "scheduled"
  },
  "receiptStatus": {
    "APPLIED": "applied",
    "SCHEDULED": "scheduled",
    "PROVISIONING": "provisioning",
    "FAILED": "failed"
  },
  "changeAction": {
    "SWITCH": "switch",
    "RENEW": "renew",
    "STOP_RENEWAL": "stopRenewal"
  },
  "sourceKind": {
    "override": "override",
    "plan": "plan",
    "addon": "addon"
  },
  "sourceState": {
    "active": "active",
    "revoked": "revoked",
    "expired": "expired",
    "scheduled": "scheduled"
  },
  "decisionKind": {
    "capability": "capability",
    "quota": "quota",
    "field": "field",
    "module": "module"
  },
  "provisioningState": {
    "QUEUED": "queued",
    "RUNNING": "running",
    "RETRY_WAIT": "retryWait",
    "READY": "ready",
    "APPLIED": "applied",
    "FAILED": "failed",
    "RECONCILIATION_REQUIRED": "reconciliationRequired",
    "CANCELLED": "cancelled"
  },
  "technicalStatus": {
    "MODULE_TECHNICAL_STATUS_READY": "ready",
    "MODULE_TECHNICAL_STATUS_NOT_READY": "notReady",
    "MODULE_TECHNICAL_STATUS_DISABLED": "disabled",
    "MODULE_TECHNICAL_STATUS_UNSPECIFIED": "unspecified",
    "ready": "ready",
    "not_ready": "notReady",
    "disabled": "disabled"
  },
  "salesStatus": {
    "MODULE_SALES_STATUS_SELLABLE": "sellable",
    "MODULE_SALES_STATUS_RETIRED": "retired",
    "MODULE_SALES_STATUS_UNSPECIFIED": "unspecified",
    "sellable": "sellable",
    "retired": "retired"
  }
} as const satisfies { [K in CommercialStateKind]: Record<CommercialStateCode<K>, string> }
