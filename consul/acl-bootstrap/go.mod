module github.com/bzub/config-functions/consul/acl-bootstrap

go 1.13

require (
	github.com/bzub/config-functions/cfunc v0.0.0-00010101000000-000000000000
	sigs.k8s.io/kustomize/kyaml v0.0.4-0.20191224180729-697a6e9759e3
)

replace github.com/bzub/config-functions/cfunc => ../../cfunc
