[vault]: https://www.vaultproject.io/
[FunctionConfig]: https://pkg.go.dev/github.com/bzub/config-functions/vault?tab=doc#FunctionConfig
[FunctionData]: https://pkg.go.dev/github.com/bzub/config-functions/vault?tab=doc#FunctionData

# Vault Configuration Function

Creates Resource configs to deploy [Vault][vault] on Kubernetes.

## Function Features

The function ConfigMap is defined in the [FunctionConfig][FunctionConfig] Go
type. The options available to configure the function are documented in the
[FunctionData][FunctionData] type.

## Getting Started

In the following example we create Resource configs for a basic, no-frills
Vault server. For production deployments, check out the [production
demo](./productionExample.md).

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/function-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-vault
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/vault:v0.0.1
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
├── [Resource]  ConfigMap example/my-vault
├── [Resource]  ConfigMap example/my-vault-server
├── [Resource]  Service example/my-vault-server
└── [Resource]  StatefulSet example/my-vault-server'

TEST="$(kustomize config tree $DEMO --graph-structure=owners)"
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
  name: my-vault
  namespace: "example"
  labels:
    app.kubernetes.io/instance: my-vault
    app.kubernetes.io/name: vault-server
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/vault:v0.0.1
data:
  generate_tls_enabled: "false"
  init_enabled: "false"
  unseal_enabled: "false"
  unseal_secret_name: "my-vault-example-unseal"'

TEST="$(cat $DEMO/function-config.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

### Metadata

The following information from the function config are applied to all Resource
configs the function manages/generates:
- `metadata.name` - Used as a prefix for Resource names.
- `metadata.namespace`

In addition, the function sets the following labels on Resource configs:
- `app.kubernetes.io/name` - Defaults to `vault-server`
- `app.kubernetes.io/instance` - Defaults to the function config's `metadata.name`

<!-- @verifyStatefulSetMetadata @test -->
```sh
EXPECTED='.
└── [Resource]  StatefulSet example/my-vault-server
    ├── metadata.labels: {app.kubernetes.io/instance: my-vault, app.kubernetes.io/name: vault-server}
    └── spec.selector: {matchLabels: {app.kubernetes.io/instance: my-vault, app.kubernetes.io/name: vault-server}}'

TEST="$(
kustomize config grep "kind=StatefulSet" $DEMO |\
kustomize config tree --field="metadata.labels" --field="spec.selector" --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

<!-- @verifyServiceMetadata @test -->
```sh
EXPECTED='.
└── [Resource]  Service example/my-vault-server
    └── spec.selector: {app.kubernetes.io/instance: my-vault, app.kubernetes.io/name: vault-server}'

TEST="$(
kustomize config grep "kind=Service" $DEMO |\
kustomize config tree --field="spec.selector" --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
