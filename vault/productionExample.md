[vault]: https://www.vaultproject.io/

# Vault Configuration Function Production Example

Creates Resource configs to deploy [Vault][vault] on Kubernetes, using the
[more advanced features](./README.md#function-features) of the Vault config
function. All Secrets necessary are generated in-cluster via Jobs.

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)
mkdir $DEMO/functions

cat <<EOF >$DEMO/functions/configmap_my-vault.yaml
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
  init_job_enabled: "true"
  unseal_job_enabled: "true"
  tls_generator_job_enabled: "true"
EOF
```

Generate Resources.
<!-- @generateInitialResources @test -->
```sh
config run $DEMO
```

## Generated Resources

The function config generates the following resources.
<!-- @verifyResourceList @test -->
```sh
EXPECTED='.
├── [Resource]  ConfigMap example/my-vault-server-cfssl
├── [Resource]  ConfigMap example/my-vault-server
├── [Resource]  ConfigMap example/my-vault
├── [Resource]  Job example/my-vault-init
├── [Resource]  Job example/my-vault-server-cfssl
├── [Resource]  Job example/my-vault-unseal
├── [Resource]  Role example/my-vault-init
├── [Resource]  Role example/my-vault-server-cfssl
├── [Resource]  Role example/my-vault-unseal
├── [Resource]  RoleBinding example/my-vault-init
├── [Resource]  RoleBinding example/my-vault-server-cfssl
├── [Resource]  RoleBinding example/my-vault-unseal
├── [Resource]  Service example/my-vault-server
├── [Resource]  ServiceAccount example/my-vault-init
├── [Resource]  ServiceAccount example/my-vault-server-cfssl
├── [Resource]  ServiceAccount example/my-vault-unseal
└── [Resource]  StatefulSet example/my-vault-server'

TEST="$(config tree --graph-structure=owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
