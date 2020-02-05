package prometheus

import (
	"fmt"
	"strings"

	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	ScrapeConfigsAnnotation = "config.bzub.dev/prometheus-scrape_configs"
)

// PrometheusFilter implements kio.Filter
type PrometheusFilter struct {
	RW *kio.ByteReadWriter
}

// Filter generates Resources.
func (f *PrometheusFilter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Workaround single line style of function config.
	if err := cfunc.FixStyles(in...); err != nil {
		return nil, err
	}

	// Get data for templates.
	fnCfg, err := f.FunctionConfig(in)
	if err != nil {
		return nil, err
	}

	// Generate a ConfigMap from the function config.
	fnConfigMap, err := cfunc.ParseTemplate("function-cm", functionCMTemplate, fnCfg)
	if err != nil {
		return nil, err
	}

	// Start building our generated Resource slice.
	generatedRs := []*yaml.RNode{fnConfigMap}

	// Generate Prometheus server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(f.serverTemplates(), fnCfg)
	if err != nil {
		return nil, err
	}
	generatedRs = append(generatedRs, serverRs...)

	// Return the generated resources + patches + input.
	return append(generatedRs, in...), nil
}

// FunctionConfig populates a struct with information needed for Resource
// templates.
func (f *PrometheusFilter) FunctionConfig(in []*yaml.RNode) (*FunctionConfig, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}
	// Make sure function config has metadata.name.
	if fnMeta.Name == "" {
		return nil, fmt.Errorf("function config must specify metadata.name.")
	}

	scrapeConfigs, err := f.getScrapeConfigs(in)
	if err != nil {
		return nil, err
	}

	// Set defaults.
	fnCfg := FunctionConfig{}
	fnCfg.Data = FunctionData{
		ScrapeConfigs: scrapeConfigs,
	}

	// Populate function data from config.
	if err := yaml.Unmarshal([]byte(f.RW.FunctionConfig.MustString()), &fnCfg); err != nil {
		return nil, err
	}

	// Set app labels.
	if fnCfg.Labels == nil {
		fnCfg.Labels = make(map[string]string)
	}
	name, ok := fnCfg.Labels["app.kubernetes.io/name"]
	if !ok || name == "" {
		fnCfg.Labels["app.kubernetes.io/name"] = "prometheus-server"
	}
	instance, ok := fnCfg.Labels["app.kubernetes.io/instance"]
	if !ok || instance == "" {
		fnCfg.Labels["app.kubernetes.io/instance"] = fnMeta.Name
	}

	return &fnCfg, nil
}

func (f *PrometheusFilter) getScrapeConfigs(in []*yaml.RNode) ([]string, error) {
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
