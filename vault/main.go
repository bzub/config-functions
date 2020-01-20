package main

import (
	"fmt"
	"os"

	"github.com/bzub/config-functions/cfunc"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// filter implements kio.Filter
type filter struct {
	rw *kio.ByteReadWriter
}

func main() {
	rw := &kio.ByteReadWriter{
		Reader:                os.Stdin,
		Writer:                os.Stdout,
		KeepReaderAnnotations: true,
	}

	err := kio.Pipeline{
		Inputs: []kio.Reader{rw},
		Filters: []kio.Filter{
			&filter{rw},
			&filters.MergeFilter{},
			&filters.FormatFilter{},
			&filters.FileSetter{},
		},
		Outputs: []kio.Writer{rw},
	}.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// Filter generates Resources.
func (f *filter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	// Workaround single line style of function config.
	if err := cfunc.FixStyles(in...); err != nil {
		return nil, err
	}

	// Get data for templates.
	fnCfg, err := f.functionConfig()
	if err != nil {
		return nil, err
	}

	// Generate a ConfigMap from the function config.
	fnConfigMap, err := cfunc.ParseTemplate("function-cm", functionCMTemplate, fnCfg)
	if err != nil {
		return nil, err
	}
	in = append(in, fnConfigMap)

	// Generate Vault server Resources from templates.
	serverRs, err := cfunc.ParseTemplates(f.serverTemplates(), fnCfg)
	if err != nil {
		return nil, err
	}
	in = append(in, serverRs...)

	if fnCfg.Data.InitEnabled {
		// Generate agent TLS Resources from templates.
		initRs, err := cfunc.ParseTemplates(f.initJobTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		in = append(in, initRs...)
	}

	if fnCfg.Data.UnsealEnabled {
		// Generate agent TLS Resources from templates.
		unsealRs, err := cfunc.ParseTemplates(f.unsealJobTemplates(), fnCfg)
		if err != nil {
			return nil, err
		}
		in = append(in, unsealRs...)
	}

	// Return the input + generated resources + patches.
	return in, nil
}

// functionConfig populates a struct with information needed for Resource
// templates.
func (f *filter) functionConfig() (*functionConfig, error) {
	fnMeta, err := f.rw.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}
	// Make sure function config has metadata.name.
	if fnMeta.Name == "" {
		return nil, fmt.Errorf("function config must specify metadata.name.")
	}

	// Set defaults.
	fnCfg := functionConfig{}
	fnCfg.Data = functionData{
		UnsealSecretName: fnMeta.Name + "-" + fnMeta.Namespace + "-unseal",
	}

	// Populate function data from config.
	if err := yaml.Unmarshal([]byte(f.rw.FunctionConfig.MustString()), &fnCfg); err != nil {
		return nil, err
	}

	// Set app labels.
	if fnCfg.Labels == nil {
		fnCfg.Labels = make(map[string]string)
	}
	name, ok := fnCfg.Labels["app.kubernetes.io/name"]
	if !ok || name == "" {
		fnCfg.Labels["app.kubernetes.io/name"] = "vault-server"
	}
	instance, ok := fnCfg.Labels["app.kubernetes.io/instance"]
	if !ok || instance == "" {
		fnCfg.Labels["app.kubernetes.io/instance"] = fnMeta.Name
	}

	return &fnCfg, nil
}
