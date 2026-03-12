module github.com/ElioNeto/vyx/cli

go 1.22

require (
	github.com/ElioNeto/vyx/scanner v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/ElioNeto/vyx/scanner => ../../scanner
