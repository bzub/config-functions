package cfssl

import (
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var functionCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  secret_name: "{{ .Data.SecretName }}"
`

// FunctionConfig holds information used in Resource templates.
type FunctionConfig struct {
	// ObjectMeta contains Resource metadata to use in templates.
	yaml.ObjectMeta `yaml:"metadata"`

	Data FunctionData
}

type FunctionData struct {
	// SecretName is the name of the Secret used to hold generated certs
	// and keys. Defaults to the metadata.name of the function config.
	SecretName string `yaml:"secret_name"`
}

func (d *FunctionData) UnmarshalYAML(node *yaml.Node) error {
	var key, value string
	for i := range node.Content {
		if key == "" {
			key = node.Content[i].Value
			continue
		}
		value = node.Content[i].Value

		// Convert KV string values into associated FunctionData types.
		switch {
		case key == "secret_name":
			d.SecretName = value
		}

		key = ""
	}

	return nil
}
