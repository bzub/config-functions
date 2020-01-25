package consul

import (
	"fmt"

	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const casiAnnotation = "config.bzub.dev/consul-agent-sidecar-injector"

func sidecarPatches(in []*yaml.RNode, fnCfg *FunctionConfig) ([]*yaml.RNode, error) {
	patches := []*yaml.RNode{}
	for _, r := range in {
		aValue, err := r.Pipe(yaml.GetAnnotation(casiAnnotation))
		if err != nil {
			return nil, err
		}
		if aValue == nil {
			continue
		}

		config, err := yaml.Parse(aValue.Document().Value)
		if err != nil {
			return nil, err
		}

		// Determine if sidecar injector config name matches this
		// Consul instance.
		cName, err := config.Pipe(yaml.Lookup("metadata", "name"))
		switch {
		case err != nil:
			return nil, err
		case cName == nil:
			return nil, fmt.Errorf("metadata.name missing in config.")
		case cName.Document().Value != fnCfg.Name:
			continue
		}

		// Determine if sidecar injector config namespace matches this
		// Consul instance.
		cNS, err := config.Pipe(yaml.Lookup("metadata", "namespace"))
		switch {
		case err != nil:
			return nil, err
		case cNS == nil:
			return nil, fmt.Errorf("metadata.namespace missing in config.")
		case cNS.Document().Value != fnCfg.Namespace:
			continue
		}

		// Create a sidecar patch config for this Resource.
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}
		patchCfg := &casiConfig{
			PatchTarget:    rMeta,
			FunctionConfig: fnCfg,
		}

		// Create a sidecar patch for this Resource.
		scPatch, err := cfunc.ParseTemplate(
			"sidecar-patch", sidecarPatchTemplate, patchCfg,
		)
		if err != nil {
			return nil, err
		}
		patches = append(patches, scPatch)

		if fnCfg.Data.TLSGeneratorJobEnabled {
			// Create a ConfigMap to configure Consul agent TLS.
			sidecarTLSCM, err := cfunc.ParseTemplate(
				"sidecar-tls-cm", sidecarTLSCMTemplate, patchCfg,
			)
			if err != nil {
				return nil, err
			}
			patches = append(patches, sidecarTLSCM)
		}
	}

	return patches, nil
}

var sidecarPatchTemplate = `apiVersion: {{ .PatchTarget.APIVersion }}
kind: {{ .PatchTarget.Kind }}
metadata:
  name: {{ .PatchTarget.Name }}
  namespace: "{{ .PatchTarget.Namespace }}"
spec:
  template:
    spec:
      containers:
        - name: consul-agent
          image: docker.io/library/consul:1.7.0-beta3
          command:
            - consul
            - agent
            - -bind=0.0.0.0
            - -config-dir=/consul/configs
            - -retry-join={{ .Name }}-server.{{ .Namespace }}.svc.cluster.local
          env:
{{ if .Data.TLSGeneratorJobEnabled }}
            - name: CONSUL_HTTP_ADDR
              value: https://127.0.0.1:8501
            - name: CONSUL_CACERT
              value: /consul/tls/consul-agent-ca.pem
            - name: CONSUL_CLIENT_CERT
              value: /consul/tls/dc1-cli-consul-0.pem
            - name: CONSUL_CLIENT_KEY
              value: /consul/tls/dc1-cli-consul-0-key.pem
{{ else }}
            - name: CONSUL_HTTP_ADDR
              value: http://127.0.0.1:8500
{{ end }}
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -ec
                - |
{{ if .Data.TLSGeneratorJobEnabled }}
                  curl \
                    --cacert $(CONSUL_CACERT) \
                    --cert $(CONSUL_CLIENT_CERT) \
                    --key $(CONSUL_CLIENT_KEY) \
                    $(CONSUL_HTTP_ADDR)/v1/status/leader 2>/dev/null |\
                  grep -E '".+"'
{{ else }}
                  curl http://127.0.0.1:8500/v1/status/leader 2>/dev/null | \
                  grep -E '".+"'
{{ end }}
          volumeMounts:
            - name: consul-data
              mountPath: /consul/data
            - name: consul-configs
              mountPath: /consul/configs
{{ if .Data.TLSGeneratorJobEnabled }}
            - name: consul-tls-secret
              mountPath: /consul/tls
{{ end }}
      volumes:
        - name: consul-data
          emptyDir: {}
        - name: consul-configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}-{{ .Namespace }}-server
                  items:
                    - key: 00-agent-defaults.hcl
                      path: 00-agent-defaults.hcl
{{ if .Data.GossipKeyGeneratorJobEnabled }}
              - secret:
                  name: {{ .Data.GossipSecretName }}
{{ end }}
{{ if .Data.TLSGeneratorJobEnabled }}
              - configMap:
                  name: {{ .Name }}-{{ .Namespace }}-tls
        - name: consul-tls-secret
          projected:
            sources:
              - secret:
                  name: {{ .Data.TLSCASecretName }}
              - secret:
                  name: {{ .Data.TLSCLISecretName }}
              - secret:
                  name: {{ .Data.TLSClientSecretName }}
{{ end }}
`

var sidecarTLSCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-{{ .Namespace }}-tls
  namespace: {{ .PatchTarget.Namespace }}
data:
  00-agent-tls.json: |-
    {
      "verify_incoming": true,
      "verify_outgoing": true,
      "ca_file": "/consul/tls/consul-agent-ca.pem",
      "cert_file": "/consul/tls/dc1-client-consul-0.pem",
      "key_file": "/consul/tls/dc1-client-consul-0-key.pem",
      "ports": {
        "http": -1,
        "https": 8501
      }
    }
`
