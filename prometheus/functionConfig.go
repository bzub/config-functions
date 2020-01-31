package prometheus

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
`

// FunctionConfig holds information used in Resource templates. It is a Go
// representation of a Kubernetes ConfigMap Resource.
type FunctionConfig struct {
	// ObjectMeta contains Resource metadata to use in templates.
	//
	// The following information from the function config are applied to
	// all Resource configs the function manages/generates:
	// - `metadata.name` (Used as a prefix for Resource names.)
	// - `metadata.namespace`
	//
	// In addition, the function sets the following labels on Resource
	// configs:
	// - `app.kubernetes.io/name` (Defaults to `prometheus-server`.)
	// - `app.kubernetes.io/instance` (Default is the value of `metadata.name`)
	yaml.ObjectMeta `yaml:"metadata"`

	// Data contains various options specific to this config function.
	Data FunctionData
}

// FunctionData holds settings used in the config function.
type FunctionData struct {
	// ScrapeConfigs are configuration snippets to be included in the
	// Prometheus `scrape_configs`. These are collected from input Resource
	// annotations.
	//
	// https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	ScrapeConfigs []string
}
