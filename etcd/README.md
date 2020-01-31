[etcd]: https://etcd.io/
[FunctionConfig]: https://pkg.go.dev/github.com/bzub/config-functions/etcd?tab=doc#FunctionConfig
[FunctionData]: https://pkg.go.dev/github.com/bzub/config-functions/etcd?tab=doc#FunctionData

# Etcd Configuration Function

Creates Resource configs to deploy [Etcd][etcd] on Kubernetes.

## Function Features

The function metadata is documented in the [FunctionConfig][FunctionConfig] Go
type. The options available to configure the function are documented in the
[FunctionData][FunctionData] type.

## Getting Started

In the following example we create Resource configs for an Etcd server. These
configs are meant to be checked into version control, so Secrets are not
included. Optionally, all necessary Secrets can be created in-cluster via Jobs
-- check out the [production demo](./productionExample.md).

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
EOF
```

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
config run $DEMO
```

## Generated Resources

The function generates the following resources.
<!-- @verifyResources @test -->
```sh
EXPECTED='.
├── [Resource]  ConfigMap example/my-etcd-server
├── [Resource]  Service example/my-etcd-server
├── [Resource]  StatefulSet example/my-etcd-server
└── [Resource]  ConfigMap example/my-etcd'

TEST="$(config tree $DEMO --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

## Configuration

### Default Function Configuration

The function adds any missing configuration fields to the function ConfigMap we
created above, populating their values with defaults.

<!-- @verifyFunctionConfigDefaults @test -->
```sh
EXPECTED='apiVersion: v1
kind: ConfigMap
metadata:
  name: my-etcd
  namespace: "example"
  labels:
    app.kubernetes.io/instance: my-etcd
    app.kubernetes.io/name: etcd-server
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/etcd:v0.0.1
data:
  tls_ca_secret_name: "my-etcd-example-tls-ca"
  tls_generator_job_enabled: "false"
  tls_root_client_secret_name: "my-etcd-example-tls-client-root"
  tls_server_secret_name: "my-etcd-example-tls-server"'

TEST="$(cat $DEMO/my-etcd_configmap.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

### Replicas

If you change the number of replicas for the Etcd StatefulSet and re-run the
config function, it can update the TLS generator job, ETCD_INITIAL_CLUSTER
config, and other configs for you.

For illustration, let's set `spec.replicas` to `3` in the previously generated
StatefulSet, and remove the old `ETCD_INITIAL_CLUSTER` setting.
<!-- $patchSTSReplicas @test -->
```sh
sed -i '/^spec:/a\  replicas: 3' $DEMO/my-etcd-server_statefulset.yaml
sed -i '/^  ETCD_INITIAL_CLUSTER:/d' $DEMO/my-etcd-server_configmap.yaml
config run $DEMO
```

The initial cluster settings should be updated.
<!-- $verifyInitialCluster3 @test -->
```sh
EXPECTED='.
└── [Resource]  ConfigMap example/my-etcd-server
    └── data.ETCD_INITIAL_CLUSTER: my-etcd-server-0=https://my-etcd-server-0.my-etcd-server:2380,my-etcd-server-1=https://my-etcd-server-1.my-etcd-server:2380,my-etcd-server-2=https://my-etcd-server-2.my-etcd-server:2380'

TEST="$(config tree \
  --field="data.ETCD_INITIAL_CLUSTER" \
  --graph-structure=owners \
  $DEMO/my-etcd-server_configmap.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
