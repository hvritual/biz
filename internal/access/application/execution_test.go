package application

import (
	"context"
	"yunka.io/framework/execution"
)

type tenantTestUnit struct{}

func (*tenantTestUnit) Commit(context.Context) error   { return nil }
func (*tenantTestUnit) Rollback(context.Context) error { return nil }
func (*tenantTestUnit) Close() error                   { return nil }

type tenantTestTransactionFactory struct{ unit execution.UnitOfWork }

func (factory tenantTestTransactionFactory) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) {
	return factory.unit, nil
}
