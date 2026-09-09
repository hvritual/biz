package policy

import (
	"context"
	"testing"

	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

type ce02GrantResolver struct{ platform bool }
func (r ce02GrantResolver) ResolveGrants(_ context.Context, request authz.GrantRequest)([]authz.Grant,error){if request.Principal.TenantID!=""{return nil,nil};out:=make([]authz.Grant,0,len(request.Permissions));for _,p:=range request.Permissions{out=append(out,authz.Grant{Permission:p,RoleID:"platform-admin"})};return out,nil}

func TestCE02PlatformAPIRejectsTenantPrincipal(t *testing.T){
	const method="/commercial.v1.ModuleCatalogApplication/CreateModule"
	resolver:=ModuleCatalogResolver();policy,ok:=resolver.ResolvePolicy(context.Background(),method);if !ok{t.Fatal("generated ModuleCatalog policy missing")};if policy.TenantRequired{t.Fatal("platform module operation must not be tenant-bound")}
	authorizer,err:=authz.NewGrantAuthorizerWithResolver(ce02GrantResolver{});if err!=nil{t.Fatal(err)};runtime,err:=authz.NewOperationRuntime(resolver,authorizer,nil);if err!=nil{t.Fatal(err)}
	platform:=identity.WithPrincipal(context.Background(),identity.Principal{Subject:"platform-admin:ce02",Roles:[]string{"platform-admin"},AuthMethod:identity.AuthMethodAPIKey,Authenticated:true});if _,err:=runtime.Prepare(platform,method,struct{}{});err!=nil{t.Fatalf("platform principal denied: %v",err)}
	tenant:=identity.WithPrincipal(context.Background(),identity.Principal{Subject:"tenant-user:ce02",TenantID:"tenant-1",UserID:"u1",Roles:[]string{"owner"},AuthMethod:identity.AuthMethodAPIKey,Authenticated:true});if _,err:=runtime.Prepare(tenant,method,struct{}{});err==nil{t.Fatal("tenant principal unexpectedly authorized for platform module API")}
}
