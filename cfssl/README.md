[cfssl]: https://github.com/cloudflare/cfssl

# CFSSL Configuration Function

Creates a Job to generate a CA, private keys and certificates signed by said
CA. The Job then creates a Secret containing the generated assets. All this is
configured via [CFSSL][cfssl] JSON configs.

## CFSSL JSON Naming

The Job uses the following naming scheme for CFSSL JSON configs:
- `config.json` (required) is passed to `cfssl gencert -config` and holds
  profiles.
- `ca_csr.json` (required) is used to configure the CA cert/key.
- `INSTANCE_PROFILE_csr.json` are CSR configs.
  - INSTANCE is a unique name for a cert/key pair.
  - PROFILE is the profile to use that was specified in `config.json`

## Kubernetes Secret

The resulting Secret that is created contains all generated CSRs/certs/keys.
The default name of the Secret is derived from the function config metadata as:
`{{ .Name }}-{{ .Namespace }}`.

This name is configurable via the `secret_name` key in the ConfigMap.

## Getting Started

Set up a workspace and define a function configuration. The JSON data will be
used to create a CA and simple client/server TLS assets.
<!-- @createFunctionConfig @test -->
```sh
DEMO=$(mktemp -d)

cat <<EOF >$DEMO/function-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-certs
  namespace: example
  annotations:
    config.kubernetes.io/function: |
      container:
        image: gcr.io/config-functions/cfssl:v0.0.1
data:
  config.json: |-
    {
        "signing": {
            "default": {
                "expiry": "168h"
            },
            "profiles": {
                "server": {
                    "expiry": "8760h",
                    "usages": [
                        "signing",
                        "key encipherment",
                        "server auth"
                    ]
                },
                "client": {
                    "expiry": "8760h",
                    "usages": [
                        "signing",
                        "key encipherment",
                        "client auth"
                    ]
                }
            }
        }
    }

  ca_csr.json: |-
    {
        "CN": "My CA",
        "key": {
            "algo": "ecdsa",
            "size": 256
        },
        "names": [
            {
                "C": "US",
                "ST": "CA",
                "L": "San Francisco"
            }
        ]
    }

  0_server_csr.json: |-
    {
        "CN": "db0",
        "hosts": [
            "example.net",
            "db.example.net",
            "db-0.example.net"
        ],
        "key": {
            "algo": "ecdsa",
            "size": 256
        },
        "names": [
            {
                "C": "US",
                "ST": "CA",
                "L": "San Francisco"
            }
        ]
    }

  0_client_csr.json: |-
    {
        "CN": "client",
        "key": {
            "algo": "ecdsa",
            "size": 256
        },
        "names": [
            {
                "C": "US",
                "ST": "CA",
                "L": "San Francisco"
            }
        ]
    }
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
├── [Resource]  ConfigMap example/my-certs
├── [Resource]  Job example/my-certs
├── [Resource]  Role example/my-certs
├── [Resource]  RoleBinding example/my-certs
└── [Resource]  ServiceAccount example/my-certs'

TEST="$(kustomize config tree $DEMO --graph-structure=owners)"
[ "$TEST" = "$EXPECTED" ]
```

## Secret Keys

In this example a `my-certs-example` Secret would be created after the Job
completes. The data available would be:
- `0-client-key.pem`
- `0-client.csr`
- `0-client.pem`
- `0-server-key.pem`
- `0-server.csr`
- `0-server.pem`
- `ca-key.pem`
- `ca.csr`
- `ca.pem`

Cleanup the demo workspace.
<!-- @cleanupWorkspace @test -->
```sh
rm -rf $DEMO
```
