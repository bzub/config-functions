[consul]: https://www.consul.io/
[FunctionConfig]: https://pkg.go.dev/github.com/bzub/config-functions/consul?tab=doc#FunctionConfig
[FunctionData]: https://pkg.go.dev/github.com/bzub/config-functions/consul?tab=doc#FunctionData

# Consul Configuration Function

Creates Resource configs to deploy [Consul][consul] on Kubernetes.

## Function Features

The function ConfigMap is defined in the [FunctionConfig][FunctionConfig] Go
type. The options available to configure the function are documented in the
[FunctionData][FunctionData] type.

## Getting Started

In the following example we create Resource configs for a basic, no-frills
Consul server. For production deployments, check out [Function
Features](#function-features) and the [production
demo](./productionExample.md).

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
EOF
```

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
kustomize config run $DEMO
```

## Generated Resources

The function generates the following resources.
<!-- @verifyResources @test -->
```sh
EXPECTED='.
├── [Resource]  ConfigMap example/my-consul
├── [Resource]  ConfigMap example/my-consul-example-server
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  Service example/my-consul-server
└── [Resource]  StatefulSet example/my-consul-server'

TEST="$(kustomize config tree $DEMO --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

## Configuration

### Default Configuration Data

The function adds any missing configuration fields to the function ConfigMap we
created above, populating their values with defaults.

<!-- @verifyFunctionConfigDefaults @test -->
```sh
EXPECTED='apiVersion: v1
kind: ConfigMap
metadata:
  name: my-consul
  namespace: "example"
  labels:
    app.kubernetes.io/instance: my-consul
    app.kubernetes.io/name: consul-server
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/consul:v0.0.3
data:
  acl_bootstrap_enabled: "false"
  acl_bootstrap_secret_name: "my-consul-example-acl"
  agent_sidecar_injector_enabled: "false"
  agent_tls_ca_secret_name: "my-consul-example-tls-ca"
  agent_tls_cli_secret_name: "my-consul-example-tls-cli"
  agent_tls_client_secret_name: "my-consul-example-tls-client"
  agent_tls_enabled: "false"
  agent_tls_server_secret_name: "my-consul-example-tls-server"
  gossip_enabled: "false"
  gossip_secret_name: "my-consul-example-gossip"'

TEST="$(cat $DEMO/function-config.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

### Metadata

The following information from the function config are applied to all Resource configs the function
manages/generates:
- `metadata.name` - Used as a prefix for Resource names.
- `metadata.namespace`

In addition, the function sets the following labels on Resource configs:
- `app.kubernetes.io/name` - Defaults to `consul-server`
- `app.kubernetes.io/instance` - Defaults to the function config's `metadata.name`

<!-- @verifyStatefulSetMetadata @test -->
```sh
EXPECTED='.
└── [Resource]  StatefulSet example/my-consul-server
    ├── metadata.labels: {app.kubernetes.io/instance: my-consul, app.kubernetes.io/name: consul-server}
    └── spec.selector: {matchLabels: {app.kubernetes.io/instance: my-consul, app.kubernetes.io/name: consul-server}}'

TEST="$(
kustomize config grep "kind=StatefulSet" $DEMO |\
kustomize config tree --field="metadata.labels" --field="spec.selector" --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

<!-- @verifyServiceMetadata @test -->
```sh
EXPECTED='.
├── [Resource]  Service example/my-consul-server-dns
│   └── spec.selector: {app.kubernetes.io/instance: my-consul, app.kubernetes.io/name: consul-server}
├── [Resource]  Service example/my-consul-server-ui
│   └── spec.selector: {app.kubernetes.io/instance: my-consul, app.kubernetes.io/name: consul-server}
└── [Resource]  Service example/my-consul-server
    └── spec.selector: {app.kubernetes.io/instance: my-consul, app.kubernetes.io/name: consul-server}'

TEST="$(
kustomize config grep "kind=Service" $DEMO |\
kustomize config tree --field="spec.selector" --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

### Setters

Some parts of the configuration can be modified via `kustomize config set`.

<!-- @listDefaultSetters @test -->
```sh
EXPECTED=\
"my-consul-replicas"

TEST="$(kustomize config set $DEMO|tail -n +2|awk '{print $1}')"
[ "$TEST" = "$EXPECTED" ]
```

#### Replicas

To change the number of Consul server replicas, it's recommended to use
`kustomize config set` because this impacts other areas of the configs.

<!-- @verifyConsulReplicas3 @test -->
```sh
kustomize config set $DEMO my-consul-replicas 3

EXPECTED='.
└── [Resource]  StatefulSet example/my-consul-server
    ├── spec.replicas: 3
    └── spec.template.spec.containers
        └── 0
            └── [name=CONSUL_REPLICAS]: {name: CONSUL_REPLICAS, value: "3"}'

TEST="$(kustomize config grep "kind=StatefulSet" $DEMO |\
  kustomize config tree --graph-structure=owners --replicas \
    --field="spec.template.spec.containers[name=consul].env[name=CONSUL_REPLICAS]")"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
