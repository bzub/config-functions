[vault]: https://www.vaultproject.io/

# Vault Configuration Function Production Example

Creates Resource configs to deploy [Vault][vault] on Kubernetes, using the more
advanced features of the Vault config function.

## Getting Started

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
data:
  init_enabled: "true"
  unseal_enabled: "true"
  generate_tls_enabled: "true"
EOF
```

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
kustomize config run $DEMO
```

## Generated Resources

The function config generates the following resources.
<!-- @verifyResourceList @test -->
```sh
EXPECTED='.
├── [Resource]  ConfigMap example/my-vault
├── [Resource]  Job example/my-vault-init
├── [Resource]  Role example/my-vault-init
├── [Resource]  RoleBinding example/my-vault-init
├── [Resource]  ServiceAccount example/my-vault-init
├── [Resource]  ConfigMap example/my-vault-server-cfssl
├── [Resource]  Job example/my-vault-server-cfssl
├── [Resource]  Role example/my-vault-server-cfssl
├── [Resource]  RoleBinding example/my-vault-server-cfssl
├── [Resource]  ServiceAccount example/my-vault-server-cfssl
├── [Resource]  ConfigMap example/my-vault-server
├── [Resource]  Service example/my-vault-server
├── [Resource]  StatefulSet example/my-vault-server
├── [Resource]  Job example/my-vault-unseal
├── [Resource]  Role example/my-vault-unseal
├── [Resource]  RoleBinding example/my-vault-unseal
└── [Resource]  ServiceAccount example/my-vault-unseal'

TEST="$(kustomize config tree --graph-structure=owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
