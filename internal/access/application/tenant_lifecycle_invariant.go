package application

import "errors"

var ErrTenantNotActive = errors.New("access: tenant is not active")
