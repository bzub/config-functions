[etcd]: https://etcd.io/

# Etcd Configuration Function Production Example

Creates Resource configs to deploy [Etcd][etcd] on Kubernetes, using the
[more advanced features](./README.md#function-features) of the Etcd config
function. All Secrets necessary are generated in-cluster via Jobs.

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/my-etcd_configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-etcd
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/etcd:v0.0.1
data:
  tls_generator_job_enabled: "true"
EOF
```

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
config run $DEMO
```

## Generated Resources

The function config generates the following resources.
<!-- @verifyResourceList @test -->
```sh
EXPECTED='.
├── [Resource]  ConfigMap example/my-etcd-cfssl
├── [Resource]  Job example/my-etcd-cfssl
├── [Resource]  Role example/my-etcd-cfssl
├── [Resource]  RoleBinding example/my-etcd-cfssl
├── [Resource]  ServiceAccount example/my-etcd-cfssl
├── [Resource]  ConfigMap example/my-etcd-server
├── [Resource]  Service example/my-etcd-server
├── [Resource]  StatefulSet example/my-etcd-server
├── [Resource]  Job example/my-etcd-tls
├── [Resource]  Role example/my-etcd-tls
├── [Resource]  RoleBinding example/my-etcd-tls
├── [Resource]  ServiceAccount example/my-etcd-tls
└── [Resource]  ConfigMap example/my-etcd'

TEST="$(config tree --graph-structure=owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
