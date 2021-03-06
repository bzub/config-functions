[consul]: https://www.consul.io/

# Consul Configuration Function Production Example

Creates Resource configs to deploy [Consul][consul] on Kubernetes, using the
[more advanced features](./README.md#function-features) of the Consul config
function. All Secrets necessary are generated in-cluster via Jobs.

## Getting Started

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
data:
  acl_bootstrap_job_enabled: "true"
  tls_generator_job_enabled: "true"
  gossip_key_generator_job_enabled: "true"
  backup_cron_job_enabled: "true"
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
├── [Resource]  ConfigMap example/my-consul-example-agent
├── [Resource]  ConfigMap example/my-consul-example-server
├── [Resource]  ConfigMap example/my-consul
├── [Resource]  CronJob example/my-consul-backup
├── [Resource]  CronJob example/my-consul-restore-secrets
├── [Resource]  CronJob example/my-consul-restore-snapshot
├── [Resource]  Job example/my-consul-acl-bootstrap
├── [Resource]  Job example/my-consul-gossip-encryption
├── [Resource]  Job example/my-consul-tls
├── [Resource]  Role example/my-consul-acl-bootstrap
├── [Resource]  Role example/my-consul-backup
├── [Resource]  Role example/my-consul-gossip-encryption
├── [Resource]  Role example/my-consul-restore-secrets
├── [Resource]  Role example/my-consul-tls
├── [Resource]  RoleBinding example/my-consul-acl-bootstrap
├── [Resource]  RoleBinding example/my-consul-backup
├── [Resource]  RoleBinding example/my-consul-gossip-encryption
├── [Resource]  RoleBinding example/my-consul-restore-secrets
├── [Resource]  RoleBinding example/my-consul-tls
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  Service example/my-consul-server
├── [Resource]  ServiceAccount example/my-consul-acl-bootstrap
├── [Resource]  ServiceAccount example/my-consul-backup
├── [Resource]  ServiceAccount example/my-consul-gossip-encryption
├── [Resource]  ServiceAccount example/my-consul-restore-secrets
├── [Resource]  ServiceAccount example/my-consul-tls
└── [Resource]  StatefulSet example/my-consul-server'

TEST="$(config tree --graph-structure=owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
