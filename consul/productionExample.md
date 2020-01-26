[consul]: https://www.consul.io/

# Consul Configuration Function Production Example

Creates Resource configs to deploy [Consul][consul] on Kubernetes, using the
[more advanced features](./README.md#function-features) of the Consul config
function.

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/function-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-consul
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/consul:v0.0.3
data:
  acl_bootstrap_job_enabled: "true"
  tls_generator_job_enabled: "true"
  gossip_key_generator_job_enabled: "true"
EOF
```

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
├── [Resource]  ConfigMap example/my-consul
├── [Resource]  Job example/my-consul-acl-bootstrap
├── [Resource]  Role example/my-consul-acl-bootstrap
├── [Resource]  RoleBinding example/my-consul-acl-bootstrap
├── [Resource]  ServiceAccount example/my-consul-acl-bootstrap
├── [Resource]  ConfigMap example/my-consul-example-agent
├── [Resource]  ConfigMap example/my-consul-example-server
├── [Resource]  Job example/my-consul-gossip-encryption
├── [Resource]  Role example/my-consul-gossip-encryption
├── [Resource]  RoleBinding example/my-consul-gossip-encryption
├── [Resource]  ServiceAccount example/my-consul-gossip-encryption
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  Service example/my-consul-server
├── [Resource]  StatefulSet example/my-consul-server
├── [Resource]  Job example/my-consul-tls
├── [Resource]  Role example/my-consul-tls
├── [Resource]  RoleBinding example/my-consul-tls
└── [Resource]  ServiceAccount example/my-consul-tls'

TEST="$(kustomize config tree --graph-structure=owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
