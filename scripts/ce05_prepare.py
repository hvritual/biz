"""Assertion-guarded source changes, removed after locked generation."""
from pathlib import Path
import json
r=Path('.')
def patch(p,old,new):
 f=r/p;s=f.read_text();assert s.count(old)==1,(p,old[:70],s.count(old));f.write_text(s.replace(old,new))
p='contracts/proto/deviceops/v1/deviceops.proto'
patch(p,'option (yunka.dsl.v1.application) = { name: "device_management" };','option (yunka.dsl.v1.application) = { name: "device_management" requires: "deviceops/site_management" };')
patch(p,'id: "device.update"\n      use_case: "update_device"\n      permissions: "device.update"','id: "device.update"\n      use_case: "update_device"\n      permissions: "device.update"\n      permissions: "site.read"\n      requires_operations: "site.validate_transfer_target"\n      composition: COMPOSITION_LOCAL')
p='internal/deviceops/application/service.go'
patch(p,'type Service struct {\n\trepositories','type Service struct {\n targets DeviceManagementToDeviceopsSiteManagementChildCapability\n\trepositories')
patch(p,'func (service *Service) ListDevices','// NewServiceWithCapabilities binds the declared conditional target lookup.\nfunc NewServiceWithCapabilities(repositories requestscope.RepositoryFactory[ports.ScopedRepositories], targets DeviceManagementToDeviceopsSiteManagementChildCapability) (*Service,error) {\n if targets==nil { return nil,errors.New("deviceops: target validation child required") }; service,err:=NewService(repositories);if err!=nil {return nil,err};service.targets=targets;return service,nil\n}\n\nfunc (service *Service) ListDevices')
patch(p,'''if siteID != current.SiteID {
			if _, err := scope.Repositories().Site.Get(scope.Context(), siteID); err != nil {
				return domain.Device{}, err
			}
		}''','''if siteID != current.SiteID {
            if service.targets==nil { return domain.Device{},errors.New("deviceops: target validation child unavailable") }
            target,err:=service.targets.ValidateTransferTarget(scope.Context(),&deviceopsv1.ValidateTransferTargetRequest{SiteId:siteID})
            if err!=nil { return domain.Device{},err }
            if target==nil || target.Id!=siteID { return domain.Device{},ErrInvalid }
        }''')
p='internal/bizruntime/runtime.go'
patch(p,'deviceapp "github.com/hvritual/biz/internal/deviceops/application"','deviceapp "github.com/hvritual/biz/internal/deviceops/application"\n deviceports "github.com/hvritual/biz/internal/deviceops/ports"\n "github.com/hvritual/biz/internal/commercial/enforcement"\n "github.com/hvritual/biz/internal/commercial/application/entitlementmanagement"')
patch(p,'httpAuthentication(authenticator, apiMux)','httpAuthentication(authenticator, enforcement.HTTP(apiMux))')
patch(p,'grpc.ChainUnaryInterceptor(grpcAuthentication(authenticator))','grpc.ChainUnaryInterceptor(grpcAuthentication(authenticator),enforcement.RPC())')
patch(p,'device             *deviceapp.Service','deviceRepositories requestscope.RepositoryFactory[deviceports.ScopedRepositories]')
patch(p,'''func (factory applicationFactories) BuildDeviceopsDeviceManagement(generatedassembly.DeviceopsDeviceManagementDependencies) (deviceapp.DeviceManagementApplication, error) {
	if factory.device == nil { return nil, errors.New("biz runtime: device management application is required") }
	return factory.device, nil
}''','''func (factory applicationFactories) BuildDeviceopsDeviceManagement(dependencies generatedassembly.DeviceopsDeviceManagementDependencies) (deviceapp.DeviceManagementApplication, error) {
 service,err:=deviceapp.NewServiceWithCapabilities(factory.deviceRepositories,dependencies.DeviceopsSiteManagement)
 if err!=nil { return nil,err };return checkedDevice{inner:service},nil
}''')
patch(p,'return factory.site, nil','return checkedSite{inner:factory.site}, nil')
patch(p,'return deviceapp.NewCrossApplicationTransferService(dependencies.DeviceopsSiteManagement, dependencies.DeviceopsDeviceManagement)','service,err:=deviceapp.NewCrossApplicationTransferService(dependencies.DeviceopsSiteManagement, dependencies.DeviceopsDeviceManagement)\n if err!=nil {return nil,err};return checkedTransfer{inner:service},nil')
patch(p,'security, err := authz.NewExecutionSecurity(grantAuthorizer, guards)', '''catalogReader,err:=modulecatalog.NewService(commercialStore,modulecatalog.ProductionRegistry())
 if err!=nil { return generatedassembly.RuntimeBindings{},err }
 sourceReader,err:=commercialpersistence.NewEntitlementStateReader(accessDatabase)
 if err!=nil { return generatedassembly.RuntimeBindings{},err }
 decisionReader,err:=entitlementmanagement.BuildDecisionReader(sourceReader,catalogReader)
 if err!=nil { return generatedassembly.RuntimeBindings{},err }
 commercialGuard,err:=enforcement.New(decisionReader,enforcement.LogAuditor{})
 if err!=nil { return generatedassembly.RuntimeBindings{},err }
 security, err := authz.NewExecutionSecurity(grantAuthorizer, commercialGuards{commercial:commercialGuard,scopes:guards})''')
patch(p,'''deviceService, err := deviceapp.NewService(deviceRepositories)
	if err != nil { return generatedassembly.RuntimeBindings{}, err }
	''','')
patch(p,'device: deviceService,','deviceRepositories: deviceRepositories,')
p='internal/deviceops/security/scope.go'
patch(p,'for _, grant := range authorized.Decision.Grants {','''resourcePermission := map[authz.OperationID]authz.PermissionKey{
 "device.list":"device.read", "device.get":"device.read", "device.create":"device.create",
 "device.update":"device.update", "device.delete":"device.delete", "device.transfer":"device.update",
 "site.validate_transfer_target":"site.read",
 }[authorized.Policy.Operation]
 if resourcePermission=="" {return nil,denied(authorized)}
 for _, grant := range authorized.Decision.Grants {
 if grant.Permission!=resourcePermission {continue}''')
d=r/'contracts/commercial/operation-capabilities.v1.json';j=json.loads(d.read_text());assert j['mapping_version']=='2';j['mapping_version']='3'
for o in j['operations']:
 if o['operation_id']=='device.update':
  assert not o['children'];o['children']=[dict(operation_id='site.validate_transfer_target',mode='conditional',condition='requested destination differs from the visible current device site',source='internal/deviceops/application/service.go#UpdateDevice')]
d.write_text(json.dumps(j,ensure_ascii=False,indent=2)+'\n')
p='integration/c9_8_cross_application_mysql_test.go'
patch(p,'deviceCapability, err := deviceapp.NewDeviceTransferToDeviceopsDeviceManagementChildCapability(deviceService, executor)', '''updateTarget,err:=deviceapp.NewDeviceManagementToDeviceopsSiteManagementChildCapability(siteService,executor)
 if err!=nil { t.Fatal(err) }
 deviceService,err=deviceapp.NewServiceWithCapabilities(repositories,updateTarget)
 if err!=nil { t.Fatal(err) }
 deviceCapability, err := deviceapp.NewDeviceTransferToDeviceopsDeviceManagementChildCapability(deviceService, executor)''')
f=r/p;s=f.read_text().replace('children["site.validate_transfer_target"] != 1','children["site.validate_transfer_target"] != 2').replace('want site.validate_transfer_target=1','want site.validate_transfer_target=2');f.write_text(s)
patch('integration/deviceops_mysql_test.go','\treturn started\n}', '\tce05LegacyFixtureGrants(t,db,tenant,[]string{"access-management","device-operations"})\n\treturn started\n}')
patch('integration/b12_member_runtime_mysql_test.go','func seedB123TenantAdmin(t *testing.T, db *gorm.DB, tenantID, userID, email, rawToken string) {','func seedB123TenantAdmin(t *testing.T, db *gorm.DB, tenantID, userID, email, rawToken string) {\n defer ce05LegacyFixtureGrants(t,db,tenantID,[]string{"access-management"})')
p='docs/commercial-entitlements/tasks.json';f=r/p;s=f.read_text();line=next(l for l in s.splitlines() if '"id":"CE-05"' in l);assert '"status":"PLANNED"' in line;f.write_text(s.replace(line,line.replace('"status":"PLANNED"','"status":"IN_PROGRESS"')))
print('CE05_PATCH=PASS')
