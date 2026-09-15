package bizruntime

import (
	accessapp "github.com/hvritual/biz/internal/access/application"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
)

func (factory applicationFactories) BuildAccessTenantAuditManagement(generatedassembly.AccessTenantAuditManagementDependencies) (accessapp.TenantAuditManagementApplication, error) {
	return accessapp.NewTenantAuditManagementService(factory.tenantRepositories)
}
