package bizruntime

import (
	accessapp "github.com/hvritual/biz/internal/access/application"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
)

func (factory applicationFactories) BuildAccessTenantAuditManagement(generatedassembly.AccessTenantAuditManagementDependencies) (accessapp.TenantAuditManagementApplication, error) {
	inner, err := accessapp.NewTenantAuditManagementService(factory.tenantRepositories)
	if err != nil {
		return nil, err
	}
	return checkedTenantAudit{inner: inner}, nil
}
