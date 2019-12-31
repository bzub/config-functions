[docs]: https://learn.hashicorp.com/consul/day-0/acl-guide

# Consul ACL Bootstrap Config Function

This function generates a Job (and associated resources) which executes `consul
acl bootstrap` on a new Consul cluster, and stores the bootstrap token
information in a Secret.

It is inspired by the official [Consul documentation][docs].

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/99-local-config.yaml
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulConfigFunction
metadata:
  name: my-consul-server
  namespace: example
  labels:
    app.kubernetes.io/instance: my-consul
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul:v0.0.1
---
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulACLBootstrapConfigFunction
metadata:
  name: my-consul-acl-bootstrap
  namespace: example
  labels:
    app.kubernetes.io/instance: my-consul
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul-acl-bootstrap:v0.0.1
EOF
```

The `app.kubernetes.io/instance` label tells the function to target `my-consul`
Resource config instances, which are managed by the `ConsulConfigFunction`
config function.

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
kustomize config run $DEMO
```

## Generated Resources

The function config generates the following resources.
<!-- @verifyResourceList @test -->
```sh
EXPECTED='.
├── [Resource]  Job example/my-consul-acl-bootstrap
├── [Resource]  Role example/my-consul-acl-bootstrap
├── [Resource]  RoleBinding example/my-consul-acl-bootstrap
├── [Resource]  ServiceAccount example/my-consul-acl-bootstrap
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  ConfigMap example/my-consul-server
├── [Resource]  Service example/my-consul-server
└── [Resource]  StatefulSet example/my-consul-server'

TEST="$(kustomize config tree --graph-structure owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
