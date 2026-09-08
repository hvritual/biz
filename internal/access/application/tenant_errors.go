package application

import "errors"

// ErrInvalidTenantRequest is shared with the hidden lifecycle implementation.
// Keeping the sentinel here preserves errors.Is identity for existing callers.
var ErrInvalidTenantRequest = errors.New("access: invalid tenant request")
