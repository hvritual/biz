package ports
import("context";"github.com/hvritual/biz/internal/commercial/domain/entitlement")
// EntitlementSnapshotReader is a derived-view capability, not a writable repository.
// Implementations join an existing root; mutation callers must use a local root.
type EntitlementSnapshotReader interface{ReadSnapshot(context.Context,string,[]string)(entitlement.Result,error)}
// Permission versions belong to Access and to a principal, never a tenant-wide
// commercial snapshot. The opaque value is only a freshness/correlation token.
type PermissionVersionReader interface{PermissionVersion(context.Context)(string,error)}
// Keys include tenant, immutable version and digest. No key means "latest".
type SnapshotCache interface{Get(context.Context,string)([]byte,error);Put(context.Context,string,[]byte)error}
