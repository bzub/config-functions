[consul]: https://www.consul.io/

# Consul Configuration Function Production Example

Creates Resource configs to deploy [Consul][consul] on Kubernetes, using the
[more advanced features](./README.md#function-features) of the Consul config
function.

## Getting Started

Set up a workspace and define a function configuration.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/function-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-consul
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/consul:v0.0.3
data:
  replicas: "3"
  agent_tls_enabled: "true"
  gossip_enabled: "true"
  acl_bootstrap_enabled: "true"
  agent_sidecar_injector_enabled: "true"
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
├── [Resource]  ConfigMap example/my-consul
├── [Resource]  Job example/my-consul-acl-bootstrap
├── [Resource]  Role example/my-consul-acl-bootstrap
├── [Resource]  RoleBinding example/my-consul-acl-bootstrap
├── [Resource]  ServiceAccount example/my-consul-acl-bootstrap
├── [Resource]  Job example/my-consul-gossip-encryption
├── [Resource]  Role example/my-consul-gossip-encryption
├── [Resource]  RoleBinding example/my-consul-gossip-encryption
├── [Resource]  ServiceAccount example/my-consul-gossip-encryption
├── [Resource]  Service example/my-consul-server-dns
├── [Resource]  Service example/my-consul-server-ui
├── [Resource]  ConfigMap example/my-consul-server
├── [Resource]  Service example/my-consul-server
├── [Resource]  StatefulSet example/my-consul-server
├── [Resource]  Job example/my-consul-tls
├── [Resource]  Role example/my-consul-tls
├── [Resource]  RoleBinding example/my-consul-tls
└── [Resource]  ServiceAccount example/my-consul-tls'

TEST="$(kustomize config tree --graph-structure=owners $DEMO)"
[ "$TEST" = "$EXPECTED" ]
```

## Sidecar Injector Annotation

Create a Deployment and re-run the config function to have a Consul agent
sidecar injected into it.
<!-- @createDeploymentForSidecar @test -->
```sh
cat <<EOF >$DEMO/my-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: other-namespace
  labels:
    app.kubernetes.io/instance: my-deployment
  annotations:
    config.bzub.dev/consul-agent-sidecar-injector: |-
      metadata:
        name: my-consul
        namespace: example
spec:
  selector:
    matchLabels:
      app.kubernetes.io/instance: my-deployment
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: my-deployment
    spec:
      containers:
        - name: example
          image: k8s.gcr.io/pause:3.1
EOF

kustomize config run $DEMO
```

The `my-deployment` Deployment now has a Consul agent sidecar container.
<!-- @verifyDeployment @test -->
```sh
EXPECTED='.
└── [my-deployment.yaml]  Deployment other-namespace/my-deployment
    └── spec.template.spec.containers
        ├── 0
        │   └── name: consul-agent
        └── 1
            └── name: example'

TEST="$(kustomize config grep "kind=Deployment" $DEMO|kustomize config tree --name)"
[ "$TEST" = "$EXPECTED" ]
```

**NOTE**: The sidecar will look for Secrets with the following name formats:
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-tls-ca`
- `{{ .ConsulName }}-{{ .ConsulNamespace }}-gossip`

These Secrets are automatically created in the Consul server's namespace.  You
will need to manually copy these Secrets to any additional namespaces where
sidecars will run.

**TODO**: Automate/simplify this.

For this example you can use kubectl/grep/sed to copy the Secrets from the
`example` namespace to the `other-namespace` namespace.

> ```sh
> kubectl -n example get secret -o yaml my-consul-example-tls-ca |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
>
> kubectl -n example get secret -o yaml my-consul-example-gossip |\
>   grep -Ev 'creationTimestamp:|resourceVersion:|selfLink:|uid:' |\
>   sed 's/namespace: example/namespace: other-namespace/' |\
>   kubectl apply -f -
> ```

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```