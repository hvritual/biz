#!/usr/bin/env bash
set -euo pipefail

: "${YUNKA_SRC:?}"
: "${RUNNER_TEMP:?}"

rm -rf internal/iam internal/site internal/device .yunka
mkdir -p internal/iam/infrastructure/persistence
mkdir -p internal/site/infrastructure/persistence
mkdir -p internal/device/infrastructure/persistence

cat > internal/iam/infrastructure/persistence/tenant.go <<'EOF'
package persistence

type TenantPO struct {
	Name string `gorm:"column:name;type:varchar(200);not null"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
EOF
cat > internal/iam/infrastructure/persistence/user.go <<'EOF'
package persistence

type UserPO struct {
	Email string `gorm:"column:email;type:varchar(320);not null;uniqueIndex"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
EOF
cat > internal/iam/infrastructure/persistence/membership.go <<'EOF'
package persistence

type MembershipPO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	Status string `gorm:"column:status;type:varchar(32);not null;index"`
}
EOF
cat > internal/iam/infrastructure/persistence/role.go <<'EOF'
package persistence

type RolePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	Name string `gorm:"column:name;type:varchar(100);not null"`
	Status string `gorm:"column:status;type:varchar(32);not null"`
}
EOF
cat > internal/iam/infrastructure/persistence/member_role.go <<'EOF'
package persistence

type MemberRolePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	RoleID string `gorm:"column:role_id;type:varchar(160);not null;index" yunka:"-"`
}
EOF
cat > internal/iam/infrastructure/persistence/role_permission.go <<'EOF'
package persistence

type RolePermissionPO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	RoleID string `gorm:"column:role_id;type:varchar(160);not null;index" yunka:"-"`
	Permission string `gorm:"column:permission;type:varchar(120);not null;index"`
	DataScope string `gorm:"column:data_scope;type:varchar(16);not null"`
}
EOF
cat > internal/iam/infrastructure/persistence/member_site.go <<'EOF'
package persistence

type MemberSitePO struct {
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	SiteID string `gorm:"column:site_id;type:varchar(64);not null;index" yunka:"-"`
}
EOF
cat > internal/iam/infrastructure/persistence/api_token.go <<'EOF'
package persistence

import "time"

type APITokenPO struct {
	TokenHash string `gorm:"column:token_hash;type:varchar(64);not null;uniqueIndex" yunka:"-"`
	ScopeTenantID string `gorm:"column:tenant_id;type:varchar(64);not null;index" yunka:"-"`
	UserID string `gorm:"column:user_id;type:varchar(64);not null;index" yunka:"-"`
	ExpiresAt *time.Time `gorm:"column:expires_at" yunka:"-"`
	Disabled bool `gorm:"column:disabled;not null;default:false" yunka:"-"`
}
EOF
cat > internal/site/infrastructure/persistence/site.go <<'EOF'
package persistence

type SitePO struct {
	Name string `gorm:"column:name;type:varchar(200);not null"`
}
EOF
cat > internal/device/infrastructure/persistence/device.go <<'EOF'
package persistence

type DevicePO struct {
	SiteID string `gorm:"column:site_id;type:varchar(64);not null;index"`
	Name string `gorm:"column:name;type:varchar(200);not null"`
	Serial string `gorm:"column:serial;type:varchar(128);not null;index"`
	CreatedBy string `gorm:"column:created_by;type:varchar(64);not null;index"`
}
EOF

gofmt -w internal
(cd "$YUNKA_SRC/app" && go build -o "$RUNNER_TEMP/yunka" ./cmd)
"$RUNNER_TEMP/yunka" init --root . --db-prefix biz
"$RUNNER_TEMP/yunka" domain new --name iam --root internal --global --no-rest --no-rpc
"$RUNNER_TEMP/yunka" domain new --name site --root internal --no-rest --no-rpc
YUNKA_DOMAIN_TOOL_DIR="$PWD/.yunka-bin" "$RUNNER_TEMP/yunka" domain new --name device --root internal
"$RUNNER_TEMP/yunka" domain generate --path internal/iam
"$RUNNER_TEMP/yunka" domain generate --path internal/site
YUNKA_DOMAIN_TOOL_DIR="$PWD/.yunka-bin" "$RUNNER_TEMP/yunka" domain generate --path internal/device
YUNKA_DOMAIN_TOOL_DIR="$PWD/.yunka-bin" "$RUNNER_TEMP/yunka" domain check --root internal

go mod edit -require=google.golang.org/grpc@v1.82.1
go mod edit -require=google.golang.org/protobuf@v1.36.11
go mod edit -replace=yunka.io/framework="$YUNKA_SRC/framework"
go mod edit -replace=yunka.io/pkg="$YUNKA_SRC/pkg"
go mod edit -replace=github.com/go-kit/kit@v0.10.0="$YUNKA_SRC/compat/go-kit-kit-log"
GOWORK=off go mod tidy
GOWORK=off go test ./...

go mod edit -replace=yunka.io/framework=../yunka.io/framework
go mod edit -replace=yunka.io/pkg=../yunka.io/pkg
go mod edit -replace=github.com/go-kit/kit@v0.10.0=../yunka.io/compat/go-kit-kit-log
rm -rf .yunka-bin
