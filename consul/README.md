[consul]: https://www.consul.io/
[gossip-encryption]: https://learn.hashicorp.com/consul/security-networking/agent-encryption
[agent-tls]: https://learn.hashicorp.com/consul/security-networking/certificates
[acl-bootstrap]: https://learn.hashicorp.com/consul/day-0/acl-guide
[agent-sidecar]: https://www.consul.io/docs/agent/basics.html

# Consul Configuration Function

Creates Resource configs to deploy [Consul][consul] on Kubernetes.

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
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  ConfigMap example/my-consul-server
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
  namespace: example
  labels:
    app.kubernetes.io/instance: my-consul
    app.kubernetes.io/name: consul-server
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/consul:v0.0.3
data:
  replicas: "1"
  acl_bootstrap_enabled: "false"
  acl_bootstrap_secret_name: "my-consul-example-acl"
  agent_sidecar_injector_enabled: "false"
  agent_tls_ca_secret_name: "my-consul-example-tls-ca"
  agent_tls_cli_secret_name: "my-consul-example-tls-cli"
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

### Replicas

If `data.replicas` is undefined in the function config, the function assumes
one replica and sets other options accordingly.
<!-- @verifyConsulReplicas1 @test -->
```sh
EXPECTED='.
└── [Resource]  StatefulSet example/my-consul-server
    ├── spec.replicas: 1
    └── spec.template.spec.containers
        └── 0
            └── [name=CONSUL_REPLICAS]: {name: CONSUL_REPLICAS, value: "1"}'

TEST="$(kustomize config grep "kind=StatefulSet" $DEMO |\
  kustomize config tree --graph-structure=owners --replicas \
    --field="spec.template.spec.containers[name=consul].env[name=CONSUL_REPLICAS]")"
[ "$TEST" = "$EXPECTED" ]
```

Set `data.replicas` in the function config and re-run the function to update
the number of StatefulSet replicas and to ensure other parts of the config get
updated as well.
<!-- @verifyConsulReplicas3 @test -->
```sh
echo '  replicas: "3"' >>$DEMO/function-config.yaml
kustomize config run $DEMO

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

## Function Features

### [Gossip Encryption Job][gossip-encryption]

With `spec.gossipEncryption.enabled=true` the Consul config function creates a
Job which creates a Consul gossip encryption key Secret, and configures a
Consul StatefulSet to use said key/Secret.

### [Agent TLS Encryption Job][agent-tls]

With `spec.agentTLSEncryption.enabled=true` the Consul config function creates
a Job which populates a Secret with Consul agent TLS assests, and configures a
Consul StatefulSet to use said Secret.

### [Automated ACL Bootstrap Job][acl-bootstrap]

With `spec.aclBootstrap.enabled=true` the Consul config function generates a
Job (and associated resources) which executes `consul acl bootstrap` on a new
Consul cluster, and stores the bootstrap token information in a Secret.

### High Availability

High Availability via Services backed by StatefulSet replicas. The function
takes care of ensuring various settings are updated depending on how you
configure the StatefulSet.

### [Agent Sidecar Injector][agent-sidecar]

With `spec.agentSidecarInjector.enabled=true` the Consul config function adds a
Consul Agent sidecar container to workload configs that contain the
`config.bzub.dev/consul-agent-sidecar-injector` annotation with a value that
targets the desired Consul server instance.

For an example see the [sidecar injector
annotation](./productionExample.md#sidecar-injector-annotation) section of the
[production example](./productionExample.md).
