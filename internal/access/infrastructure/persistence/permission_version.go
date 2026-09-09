package persistence
import("context";"crypto/sha256";"encoding/hex";"encoding/json";"yunka.io/framework/core/identity")
// PermissionVersion is a principal-specific Access projection, never a grant.
// UNION ALL binds one statement snapshot without a grants x sites Cartesian join.
func(s *Store)PermissionVersion(ctx context.Context)(string,error){
 p,ok:=identity.FromContext(ctx);if !ok||!p.Authenticated||p.TenantID==""||p.UserID==""{return "",ErrUnauthorized}
 var rows []struct{Kind,Key,Value string;Version uint64}
 result:=s.database.WithContext(ctx).Raw(`
 SELECT 'identity' AS kind,m.user_id AS `+"`key`"+`,CONCAT(t.status,'/',m.status,'/',u.status) AS value,m.version AS version
 FROM biz_memberships m JOIN biz_tenants t ON t.id=m.tenant_id JOIN biz_users u ON u.id=m.user_id WHERE m.tenant_id=? AND m.user_id=?
 UNION ALL SELECT 'tenant',id,status,version FROM biz_tenants WHERE id=?
 UNION ALL SELECT 'role',r.id,r.status,r.version FROM biz_member_roles mr JOIN biz_roles r ON r.id=mr.role_id AND r.tenant_id=mr.tenant_id WHERE mr.tenant_id=? AND mr.user_id=?
 UNION ALL SELECT 'grant',g.role_id,CONCAT(g.permission,'/',g.scope),0 FROM biz_member_roles mr JOIN biz_permission_grants g ON g.role_id=mr.role_id AND g.tenant_id=mr.tenant_id WHERE mr.tenant_id=? AND mr.user_id=?
 UNION ALL SELECT 'site',site_id,'',0 FROM biz_member_sites WHERE tenant_id=? AND user_id=?
 ORDER BY kind,`+"`key`"+`,value,version`,p.TenantID,p.UserID,p.TenantID,p.TenantID,p.UserID,p.TenantID,p.UserID,p.TenantID,p.UserID).Scan(&rows)
 if result.Error!=nil{return "",result.Error};active:=false;for _,r:=range rows{if r.Kind=="identity"&&r.Value=="active/active/active"{active=true}}
 if !active{return "",ErrUnauthorized}
 b,err:=json.Marshal(struct{Tenant,Subject string;Facts any}{p.TenantID,p.Subject,rows});if err!=nil{return "",err};sum:=sha256.Sum256(b);return "sha256:"+hex.EncodeToString(sum[:]),nil
}
