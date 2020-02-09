[consul]: https://www.consul.io/
[ConfigFunction]: https://pkg.go.dev/github.com/bzub/config-functions/consul?tab=doc#ConfigFunction
[Options]: https://pkg.go.dev/github.com/bzub/config-functions/consul?tab=doc#Options

# Consul Configuration Function

Creates Resource configs to deploy [Consul][consul] on Kubernetes.

## Function Features

Function settings are documented in the [Options][Options] Go type. Metadata
and other data is documented in the [ConfigFunction][ConfigFunction] type.

## Getting Started

In the following example we create Resource configs for a Consul server. These
configs are meant to be checked into version control, so Secrets are not
included. Optionally, all necessary Secrets can be created in-cluster via Jobs
-- check out the [production demo](./productionExample.md).

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)
mkdir $DEMO/functions

cat <<EOF >$DEMO/functions/configmap_my-consul.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-consul
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/consul:v0.0.1
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
├── [Resource]  ConfigMap example/my-consul-example-agent
├── [Resource]  ConfigMap example/my-consul-example-server
├── [Resource]  ConfigMap example/my-consul
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  Service example/my-consul-server
└── [Resource]  StatefulSet example/my-consul-server'

TEST="$(config tree $DEMO --graph-structure=owners)"
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
        image: gcr.io/config-functions/consul:v0.0.1
data:
  acl_bootstrap_job_enabled: "false"
  acl_bootstrap_secret_name: "my-consul-example-acl"
  agent_sidecar_injector_enabled: "false"
  gossip_key_generator_job_enabled: "false"
  gossip_secret_name: "my-consul-example-gossip"
  tls_ca_secret_name: "my-consul-example-tls-ca"
  tls_cli_secret_name: "my-consul-example-tls-cli"
  tls_client_secret_name: "my-consul-example-tls-client"
  tls_generator_job_enabled: "false"
  tls_server_secret_name: "my-consul-example-tls-server"'

TEST="$(cat $DEMO/functions/configmap_my-consul.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

### Setters

Some parts of the configuration can be modified via `config set`.

<!-- @listDefaultSetters @test -->
```sh
EXPECTED=\
"my-consul-replicas"

TEST="$(config set $DEMO|tail -n +2|awk '{print $1}')"
[ "$TEST" = "$EXPECTED" ]
```

#### Replicas

To change the number of Consul server replicas, it's recommended to use
`config set` because this impacts other areas of the configs.

<!-- @verifyConsulReplicas3 @test -->
```sh
config set $DEMO my-consul-replicas 3

EXPECTED='.
└── [Resource]  StatefulSet example/my-consul-server
    ├── spec.replicas: 3
    └── spec.template.spec.containers
        └── 0
            └── [name=CONSUL_REPLICAS]: {name: CONSUL_REPLICAS, value: "3"}'

TEST="$(config grep "kind=StatefulSet" $DEMO |\
  config tree --graph-structure=owners --replicas \
    --field="spec.template.spec.containers[name=consul].env[name=CONSUL_REPLICAS]")"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
