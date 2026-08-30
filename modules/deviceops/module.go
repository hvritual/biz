package deviceops

import "errors"

const ModuleName = "deviceops"

// Module is the typed capability participant declared by the generated module
// descriptor. Process/runtime assembly is owned by generated C10 Bootstrap and
// core.App rather than by this module instance.
type Module struct{}

func NewModule(dependencies Dependencies) (*Module, error) {
	if dependencies.Logger == nil {
		return nil, errors.New("deviceops: logger is required")
	}
	if dependencies.PrimaryDatabase == nil {
		return nil, errors.New("deviceops: primary database is required")
	}
	if err := dependencies.Config.Validate(); err != nil {
		return nil, err
	}
	return &Module{}, nil
}

func (*Module) Name() string { return ModuleName }
