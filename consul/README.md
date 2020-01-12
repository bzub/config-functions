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
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/consul:v0.0.2
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
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  ConfigMap example/my-consul-server
├── [Resource]  Service example/my-consul-server
└── [Resource]  StatefulSet example/my-consul-server'

TEST="$(kustomize config tree $DEMO --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

## Configuration

### Replicas

The function does not set `.spec.replicas` on the Consul StatefulSet. If
replicas is undefined, the function assumes one replica and sets other options
accordingly.
<!-- @verifyConsulReplicas1 @test -->
```sh
EXPECTED='.
└── [Resource]  StatefulSet example/my-consul-server
    └── spec.template.spec.containers
        └── 0
            └── [name=CONSUL_REPLICAS]: {name: CONSUL_REPLICAS, value: "1"}'

TEST="$(kustomize config grep "kind=StatefulSet" $DEMO |\
  kustomize config tree --graph-structure=owners --replicas \
    --field="spec.template.spec.containers[name=consul].env[name=CONSUL_REPLICAS]")"
[ "$TEST" = "$EXPECTED" ]
```

If you change the number of replicas in the generated Resource config, re-run
the function to ensure other parts of the config get updated as well.
<!-- @verifyConsulReplicas3 @test -->
```sh
# Add a replicas field to the StatefulSet spec.
sed -i '/^spec:$/a \  replicas: 3' $DEMO/my-consul-server_statefulset.yaml
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
