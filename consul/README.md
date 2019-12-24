# consul Configuration Function

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/local-config.yaml
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulLocalConfig
metadata:
  name: my-consul
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul:v0.0.1
spec:
  replicas: 1
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
ConfigMap: 1
Service: 3
StatefulSet: 1"

TEST="$(kustomize config cat $DEMO | kustomize config count)"
[ "$TEST" = "$EXPECTED" ]
```

## Configuration

### Replicas

The StatefulSet has one replica as defined in the function spec.
<!-- @verifyStatefulSetReplicas1 @test -->
```sh
EXPECTED="StatefulSet: 1"

TEST="$(kustomize config grep "spec.replicas=1" $DEMO |\
  kustomize config grep "spec.template.spec.containers[name=consul].env[name=CONSUL_REPLICAS].value=1" |\
  kustomize config count)"
[ "$TEST" = "$EXPECTED" ]
```

For high availability, increase the number of replicas in the generated
StatefulSet config. The `-bootstrap-expect` flag passed to consul will be
automatically updated via the CONSUL_REPLICAS environment variable.
<!-- @verifyStatefulSetReplicas3 @test -->
```sh
sed -i 's/replicas: 1/replicas: 3/' "$DEMO/local-config.yaml"
kustomize config run $DEMO

EXPECTED="StatefulSet: 1"

TEST="$(kustomize config grep "spec.replicas=3" $DEMO |\
  kustomize config grep "spec.template.spec.containers[name=consul].env[name=CONSUL_REPLICAS].value=3" |\
  kustomize config count)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
