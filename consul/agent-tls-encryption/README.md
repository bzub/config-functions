[docs]: https://learn.hashicorp.com/consul/security-networking/certificates

# Consul Agent TLS Encryption Config Function

This function creates a Job which populates a Secret with Consul agent TLS
assests, and configures a Consul StatefulSet to use said Secret.

It is inspired by the official [Consul documentation][docs].

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/local-config.yaml
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulConfigFunction
metadata:
  name: my-consul
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul:v0.0.1
spec:
  replicas: 1
---
apiVersion: config.kubernetes.io/v1beta1
kind: ConsulAgentTLSEncryptionConfigFunction
metadata:
  name: my-consul-agent-tls-encryption
  annotations:
    config.kubernetes.io/local-config: "true"
  configFn:
    container:
      image: gcr.io/config-functions/consul-agent-tls-encryption:v0.0.1
spec:
  statefulSetName: my-consul
EOF
```

Generate Resources from `local-config.yaml`.
<!-- @generateInitialResources @test -->
```sh
kustomize config run $DEMO
```

## Generated Resources

The function config generates the following resources.
<!-- @verifyResourceList @test -->
```sh
EXPECTED="\
.
├── [Resource]  Job my-consul-agent-tls-encryption
├── [Resource]  Role my-consul-agent-tls-encryption
├── [Resource]  RoleBinding my-consul-agent-tls-encryption
├── [Resource]  ServiceAccount my-consul-agent-tls-encryption
├── [Resource]  Service my-consul-dns
├── [Resource]  Service my-consul-ui
├── [Resource]  ConfigMap my-consul
├── [Resource]  Service my-consul
└── [Resource]  StatefulSet my-consul"

TEST="$(kustomize config tree --graph-structure owners $DEMO)"
echo "${TEST}"
[ "$TEST" = "$EXPECTED" ]
```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
