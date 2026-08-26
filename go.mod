module github.com/hvritual/biz

go 1.25.0

toolchain go1.25.13

require (
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	gorm.io/gorm v1.25.5
	yunka.io/framework v0.0.0
	yunka.io/pkg v0.0.0
)

replace yunka.io/framework => ../yunka.io/framework
replace yunka.io/pkg => ../yunka.io/pkg
replace github.com/go-kit/kit v0.10.0 => ../yunka.io/compat/go-kit-kit-log
