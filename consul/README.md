[consul]: https://www.consul.io/

# consul Configuration Function

Creates Resource configs to deploy [Consul][consul] on Kubernetes.

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/99-local-config.yaml
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulConfigFunction
metadata:
  name: my-consul
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul:v0.0.1
EOF
```

Generate Resources from `local-config.yaml`.
<!-- @generateInitialResources @test -->
```sh
kustomize config run $DEMO
```

## Generated Resources

The function generates the following resources.
<!-- @verifyResourceCounts @test -->
```sh
EXPECTED="\
.
├── [Resource]  Service my-consul-dns
├── [Resource]  Service my-consul-ui
├── [Resource]  ConfigMap my-consul
├── [Resource]  Service my-consul
└── [Resource]  StatefulSet my-consul"

TEST="$(kustomize config tree --graph-structure=owners $DEMO)"
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
└── [Resource]  StatefulSet my-consul
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
sed -i '/^spec:$/a \  replicas: 3' $DEMO/my-consul_statefulset.yaml
kustomize config run $DEMO

EXPECTED='.
└── [Resource]  StatefulSet my-consul
    ├── spec.replicas: 3
    └── spec.template.spec.containers
        └── 0
            └── [name=CONSUL_REPLICAS]: {name: CONSUL_REPLICAS, value: "3"}'

TEST="$(kustomize config grep "kind=StatefulSet" $DEMO |\
  kustomize config tree --graph-structure=owners --replicas \
    --field="spec.template.spec.containers[name=consul].env[name=CONSUL_REPLICAS]")"
echo "${TEST}"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
