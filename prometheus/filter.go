package prometheus

import (
	"strings"

	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const ScrapeConfigsAnnotation = "config.bzub.dev/prometheus-scrape_configs"

const DefaultAppNameAnnotationValue = "prometheus-server"

const functionCMTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
`

// ConfigFunction implements kio.Filter and holds information used in
// Resource templates.
type ConfigFunction struct {
	cfunc.ConfigFunction `yaml:",inline"`

	// Data contains various options specific to this config function.
	Data Options
}

// Options holds settings used in the config function.
type Options struct {
	// ScrapeConfigs are configuration snippets to be included in the
	// Prometheus `scrape_configs`. These are collected from input Resource
	// annotations.
	//
	// https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
	ScrapeConfigs []string
}

// Filter generates Resources.
func (f *ConfigFunction) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Workaround single line style of function config.
	if err := cfunc.FixStyles(in...); err != nil {
		return nil, err
	}

	// Get data for templates.
	if err := f.syncData(in); err != nil {
		return nil, err
	}

	// Generate a ConfigMap from the function config.
	fnConfigMap, err := cfunc.ParseTemplate("function-cm", functionCMTemplate, f)
	if err != nil {
		return nil, err
	}

	// Start building our generated Resource slice.
	generatedRs := []*yaml.RNode{fnConfigMap}

	// Generate Prometheus server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(serverTemplates(), f)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// syncData populates a struct with information needed for Resource templates.
func (f *ConfigFunction) syncData(in []*yaml.RNode) error {
	if err := f.SyncMetadata(DefaultAppNameAnnotationValue); err != nil {
		return err
	}

	scrapeConfigs, err := f.getScrapeConfigs(in)
	if err != nil {
		return err
	}

	// Set defaults.
	f.Data = Options{
		ScrapeConfigs: scrapeConfigs,
	}

	// Populate function data from config.
	if err := yaml.Unmarshal([]byte(f.RW.FunctionConfig.MustString()), f); err != nil {
		return err
	}

	return nil
}

func (f *ConfigFunction) getScrapeConfigs(in []*yaml.RNode) ([]string, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	scrapeConfigs := []string{}
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}
		if rMeta.Namespace != fnMeta.Namespace {
			continue
		}

		configs, err := r.Pipe(yaml.GetAnnotation(ScrapeConfigsAnnotation))
		if err != nil {
			return nil, err
		}
		if configs == nil {
			continue
		}

		c := Indent(configs.Document().Value, "      ")
		scrapeConfigs = append(scrapeConfigs, c)
	}

	return scrapeConfigs, nil
}

// indents a block of text with an indent string
func Indent(text, indent string) string {
	if text[len(text)-1:] == "\n" {
		result := ""
		for _, j := range strings.Split(text[:len(text)-1], "\n") {
			result += indent + j + "\n"
		}
		return result
	}
	result := ""
	for _, j := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		result += indent + j + "\n"
	}
	return result[:len(result)-1]
}
