[vault]: https://www.vaultproject.io/
[FunctionConfig]: https://pkg.go.dev/github.com/bzub/config-functions/vault?tab=doc#FunctionConfig
[FunctionData]: https://pkg.go.dev/github.com/bzub/config-functions/vault?tab=doc#FunctionData

# Vault Configuration Function

Creates Resource configs to deploy [Vault][vault] on Kubernetes.

## Function Features

The function metadata is documented in the [FunctionConfig][FunctionConfig] Go
type. The options available to configure the function are documented in the
[FunctionData][FunctionData] type.

## Getting Started

In the following example we create Resource configs for a Vault server. These
configs are meant to be checked into version control, so Secrets are not
included. Optionally, all necessary Secrets can be created in-cluster via Jobs
-- check out the [production demo](./productionExample.md).

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
  init_job_enabled: "false"
  tls_generator_job_enabled: "false"
  unseal_job_enabled: "false"
  unseal_secret_name: "my-vault-example-unseal"'

TEST="$(cat $DEMO/function-config.yaml)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
