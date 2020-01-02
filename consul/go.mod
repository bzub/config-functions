module github.com/bzub/config-functions/consul

go 1.13

require (
	github.com/bzub/config-functions/cfunc v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kustomize/kyaml v0.0.5-0.20200102175141-3577a7e174c3
)

replace github.com/bzub/config-functions/cfunc => ../cfunc
