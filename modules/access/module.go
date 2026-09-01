package access

import "fmt"

const ModuleName = "access"

type Module struct {
	dependencies Dependencies
}

func NewModule(dependencies Dependencies) (*Module, error) {
	if dependencies.PrimaryDatabase == nil {
		return nil, fmt.Errorf("%s: database primary is required", ModuleName)
	}
	return &Module{dependencies: dependencies}, nil
}

func (*Module) Name() string { return ModuleName }
