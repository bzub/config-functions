[docs]: https://learn.hashicorp.com/consul/day-0/acl-guide

# Consul ACL Bootstrap Config Function

This function generates a Job (and associated resources) which executes `consul
acl bootstrap` on a new Consul cluster, and stores the bootstrap token
information in a Secret.

It is inspired by the official [Consul documentation](docs).

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/local-config.yaml
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulConfigFunction
metadata:
  name: my-consul
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul:v0.0.1
spec:
  replicas: 1
---
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulACLBootstrapConfigFunction
metadata:
  name: my-consul-acl-bootstrap
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul-acl-bootstrap:v0.0.1
spec:
  serviceName: my-consul-dns
EOF
```

Generate Resources from `local-config.yaml`.
<!-- @generateInitialResources @test -->
```sh
kustomize config run $DEMO
```

## Generated Resources

The function config generates the following resources.
<!-- @verifyResourceList @test -->
```sh
EXPECTED="\
.
├── [Resource]  Job my-consul-acl-bootstrap
├── [Resource]  Role my-consul-acl-bootstrap
├── [Resource]  RoleBinding my-consul-acl-bootstrap
├── [Resource]  ServiceAccount my-consul-acl-bootstrap
├── [Resource]  Service my-consul-dns
├── [Resource]  Service my-consul-ui
├── [Resource]  ConfigMap my-consul
├── [Resource]  Service my-consul
└── [Resource]  StatefulSet my-consul"

TEST="$(kustomize config tree --graph-structure owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
