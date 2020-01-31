[nodeexporter]: https://github.com/prometheus/node_exporter
[FunctionConfig]: https://pkg.go.dev/github.com/bzub/config-functions/nodeexporter?tab=doc#FunctionConfig

# Node Exporter Configuration Function

Creates Resource configs to deploy [Node Exporter][nodeexporter] on Kubernetes.

## Function Features

The function metadata is documented in the [FunctionConfig][FunctionConfig] Go
type.

## Getting Started

In the following example we create Resource configs for a Node Exporter
DaemonSet.

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/my-nodeexporter_configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-nodeexporter
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/nodeexporter:v0.0.1
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
├── [Resource]  DaemonSet example/my-nodeexporter-server
├── [Resource]  Service example/my-nodeexporter-server
└── [Resource]  ConfigMap example/my-nodeexporter'

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
  name: my-nodeexporter
  namespace: "example"
  labels:
    app.kubernetes.io/instance: my-nodeexporter
    app.kubernetes.io/name: nodeexporter-server
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/nodeexporter:v0.0.1'

TEST="$(cat $DEMO/my-nodeexporter_configmap.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
