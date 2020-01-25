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

// functionConfig holds information used in Resource templates.
type functionConfig struct {
	// ObjectMeta contains Resource metadata to use in templates.
	yaml.ObjectMeta `yaml:"metadata"`

	Data functionData
}

type functionData struct {
	// SecretName is the name of the Secret used to hold generated certs
	// and keys. Defaults to the metadata.name of the function config.
	SecretName string `yaml:"secret_name"`
}

func (d *functionData) UnmarshalYAML(node *yaml.Node) error {
	var key, value string
	for i := range node.Content {
		if key == "" {
			key = node.Content[i].Value
			continue
		}
		value = node.Content[i].Value

		// Convert KV string values into associated functionData types.
		switch {
		case key == "secret_name":
			d.SecretName = value
		}

		key = ""
	}

	return nil
}
