//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
)

func ce293VerifyRoleReceiptRollback(t *testing.T, db *gorm.DB, tenant string) {
	t.Helper()
	role := domain.NewRole("rollback-"+ce04Random(t), tenant, "rollback receipt", time.Now().UTC())
	key, fingerprint := strings.Repeat("c", 64), strings.Repeat("d", 64)
	outside, err := persistence.NewTenantRoleRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outside.CreateOnce(context.Background(), &role, key, fingerprint); err == nil {
		t.Fatal("receipt creation accepted a nontransactional connection")
	}
	rollback := errors.New("injected root rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		repository, err := persistence.NewTenantRoleRepository(tx)
		if err != nil {
			return err
		}
		if _, err := repository.CreateOnce(context.Background(), &role, key, fingerprint); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("root rollback was not exercised: %v", err)
	}
	for table, query := range map[string]string{"biz_roles": "id", "biz_role_creation_receipts": "receipt_key"} {
		value := role.ID
		if query == "receipt_key" {
			value = key
		}
		var count int64
		if err := db.Table(table).Where(query+"=?", value).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("rollback left %s row: %d %v", table, count, err)
		}
	}
	// A completed claim marker alone must not authorize recreating a receipt.
	err = db.Transaction(func(tx *gorm.DB) error {
		repository, err := persistence.NewTenantRoleRepository(tx)
		if err != nil {
			return err
		}
		_, err = repository.CreateOnce(ports.WithRoleCreationReplay(context.Background()), &role, key, fingerprint)
		return err
	})
	if !errors.Is(err, ports.ErrTenantRoleConflict) {
		t.Fatalf("missing completed receipt did not fail closed: %v", err)
	}
}
