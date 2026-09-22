//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func TestEnterprise180DataPolicyAuthorityAndRoleReference(t *testing.T){
	db:=openDB(t);stamp:=fmt.Sprint(time.Now().UnixNano());started:=startB123Runtime(t,db)
	tenantA,tenantB:="enterprise180-policy-a-"+stamp,"enterprise180-policy-b-"+stamp
	adminA,adminB:="enterprise180-admin-a-"+stamp,"enterprise180-admin-b-"+stamp
	tokenA,tokenB:="enterprise180-token-a-"+stamp,"enterprise180-token-b-"+stamp
	seedB123TenantAdmin(t,db,tenantA,adminA,adminA+"@example.invalid",tokenA);seedB123TenantAdmin(t,db,tenantB,adminB,adminB+"@example.invalid",tokenB)
	seedB124RoleAdminPermissions(t,db,tenantA);seedB124RoleAdminPermissions(t,db,tenantB)
	now:=time.Now().UTC();siteA,siteA2,siteB:="site-a-"+stamp,"site-a2-"+stamp,"site-b-"+stamp
	for _,s:=range []struct{tenant,id string}{{tenantA,siteA},{tenantA,siteA2},{tenantB,siteB}}{if err:=db.Exec("INSERT INTO biz_deviceops_site (id,tenant_id,name,version,created_at,updated_at) VALUES (?,?,?,?,?,?)",s.id,s.tenant,s.id,1,now,now).Error;err!=nil{t.Fatal(err)}}
	dialCtx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();conn,err:=grpc.DialContext(dialCtx,started.GRPCAddress(),grpc.WithTransportCredentials(insecure.NewCredentials()),grpc.WithBlock());if err!=nil{t.Fatal(err)};defer conn.Close();client:=accessv1.NewTenantRolePermissionApplicationClient(conn)
	ctx:=func(token,key string)context.Context{return metadata.AppendToOutgoingContext(context.Background(),"authorization","Bearer "+token,"idempotency-key",key+":"+stamp)}
	policyA,err:=client.CreateTenantDataPolicy(ctx(tokenA,"create-a"),&accessv1.CreateTenantDataPolicyRequest{Name:"A limited sites",SiteIds:[]string{siteA,siteA2}});if err!=nil{t.Fatal(err)};if policyA.GetVersion()!=1||!policyA.GetEffective(){t.Fatalf("policyA=%+v",policyA)}
	if _,err=client.CreateTenantDataPolicy(ctx(tokenA,"cross-site"),&accessv1.CreateTenantDataPolicyRequest{Name:"cross",SiteIds:[]string{siteB}});err==nil{t.Fatal("tenant A accepted tenant B site")}
	policyB,err:=client.CreateTenantDataPolicy(ctx(tokenB,"create-b"),&accessv1.CreateTenantDataPolicyRequest{Name:"B",SiteIds:[]string{siteB}});if err!=nil{t.Fatal(err)}
	roleA,err:=client.CreateTenantRole(ctx(tokenA,"create-role"),&accessv1.CreateTenantRoleRequest{Name:"Policy Reader"});if err!=nil{t.Fatal(err)}
	if _,err=client.SetTenantRoleDataPolicy(ctx(tokenA,"cross-policy"),&accessv1.SetTenantRoleDataPolicyRequest{RoleId:roleA.GetId(),Version:roleA.GetVersion(),PolicyId:policyB.GetId(),PolicyVersion:policyB.GetVersion()});err==nil{t.Fatal("tenant A accepted tenant B policy")}
	roleA,err=client.SetTenantRoleDataPolicy(ctx(tokenA,"bind"),&accessv1.SetTenantRoleDataPolicyRequest{RoleId:roleA.GetId(),Version:roleA.GetVersion(),PolicyId:policyA.GetId(),PolicyVersion:policyA.GetVersion()});if err!=nil{t.Fatal(err)}
	ref:=roleA.GetDataPolicy();if ref==nil||ref.GetPolicyId()!=policyA.GetId()||ref.GetPolicyVersion()!=1||ref.GetAcceptedVersion()!=1||!ref.GetEffective(){t.Fatalf("bound ref=%+v",ref)}
	if _,err=client.SetTenantRoleDataPolicy(ctx(tokenA,"stale-role"),&accessv1.SetTenantRoleDataPolicyRequest{RoleId:roleA.GetId(),Version:roleA.GetVersion()-1,PolicyId:policyA.GetId(),PolicyVersion:policyA.GetVersion()});err==nil{t.Fatal("stale role CAS succeeded")}
	policyA,err=client.UpdateTenantDataPolicy(ctx(tokenA,"shrink"),&accessv1.UpdateTenantDataPolicyRequest{PolicyId:policyA.GetId(),Version:policyA.GetVersion(),Name:"A one site",SiteIds:[]string{siteA}});if err!=nil{t.Fatal(err)}
	roleRead,err:=client.GetTenantRole(ctx(tokenA,"read-role"),&accessv1.GetTenantRoleRequest{RoleId:roleA.GetId()});if err!=nil{t.Fatal(err)};ref=roleRead.GetDataPolicy();if ref==nil||ref.GetPolicyVersion()!=2||ref.GetAcceptedVersion()!=1||!ref.GetEffective(){t.Fatalf("readback=%+v",ref)}
	expired,err:=client.CreateTenantDataPolicy(ctx(tokenA,"expired"),&accessv1.CreateTenantDataPolicyRequest{Name:"expired",SiteIds:[]string{siteA},ExpiresAt:time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)});if err!=nil{t.Fatal(err)};if expired.GetEffective(){t.Fatal("expired policy effective")}
	role2,err:=client.CreateTenantRole(ctx(tokenA,"create-role2"),&accessv1.CreateTenantRoleRequest{Name:"Expired Ref"});if err!=nil{t.Fatal(err)};if _,err=client.SetTenantRoleDataPolicy(ctx(tokenA,"bind-expired"),&accessv1.SetTenantRoleDataPolicyRequest{RoleId:role2.GetId(),Version:role2.GetVersion(),PolicyId:expired.GetId(),PolicyVersion:expired.GetVersion()});err==nil{t.Fatal("expired policy reference accepted")}
	policyA,err=client.RevokeTenantDataPolicy(ctx(tokenA,"revoke"),&accessv1.RevokeTenantDataPolicyRequest{PolicyId:policyA.GetId(),Version:policyA.GetVersion()});if err!=nil{t.Fatal(err)};if policyA.GetEffective()||policyA.GetInvalidReason()!="revoked"{t.Fatalf("revoked=%+v",policyA)}
	roleRead,err=client.GetTenantRole(ctx(tokenA,"read-revoked"),&accessv1.GetTenantRoleRequest{RoleId:roleA.GetId()});if err!=nil{t.Fatal(err)};if roleRead.GetDataPolicy()==nil||roleRead.GetDataPolicy().GetEffective()||roleRead.GetDataPolicy().GetInvalidReason()!="revoked"{t.Fatalf("revoked ref=%+v",roleRead.GetDataPolicy())}
	if _,err=client.GetTenantDataPolicy(ctx(tokenA,"cross-read"),&accessv1.GetTenantDataPolicyRequest{PolicyId:policyB.GetId()});err==nil{t.Fatal("tenant A read tenant B policy")}
}
