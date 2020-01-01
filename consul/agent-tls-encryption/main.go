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
	*cfunc.CFunc
}

// templateData holds information used in Resource templates.
type templateData struct {
	// ObjectMeta contains Resource metadata to use in templates.
	*yaml.ObjectMeta

	// StatefulSetName is the name of the Consul server StatefulSet to
	// target for patching.
	StatefulSetName string

	// ConfigMapName is the name of the Consul server ConfigMap to target
	// for patching.
	ConfigMapName string
}

func main() {
	rw := &kio.ByteReadWriter{
		Reader:                os.Stdin,
		Writer:                os.Stdout,
		KeepReaderAnnotations: true,
	}

	filter := &filter{}
	filter.CFunc = &cfunc.CFunc{}
	filter.RW = rw

	err := kio.Pipeline{
		Inputs: []kio.Reader{rw},
		Filters: []kio.Filter{
			filter,
			&filters.MergeFilter{},
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
	// Verify and name function config metadata.
	if err := f.VerifyMeta("consul-agent-tls-encryption"); err != nil {
		return nil, err
	}

	// Get resources we care about from input.
	managedRs, err := f.ManagedResources(in)
	if err != nil {
		return nil, err
	}

	// Get data for templates.
	data, err := f.templateData(managedRs)
	if err != nil {
		return nil, err
	}

	// Generate Resources from templates.
	templateRs, err := cfunc.ParseTemplates(f.defaultTemplates(), data)
	if err != nil {
		return nil, err
	}

	// Set function config metadata on generated Resources.
	if err := f.SetMetadata(templateRs); err != nil {
		return nil, err
	}

	// Merge our templated Resources into the input Resources.
	return append(in, templateRs...), nil
}

// templateData populates a struct with information needed for Resource
// templates.
func (f *filter) templateData(in []*yaml.RNode) (*templateData, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	// Find the name of the Consul server StatefulSet and ConfigMap to
	// patch.
	var stsName, cmName string
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		switch rMeta.Kind {
		case "StatefulSet":
			stsName = rMeta.Name
		case "ConfigMap":
			cmName = rMeta.Name
		}
	}
	if stsName == "" {
		return nil, fmt.Errorf(
			"statefulset instance %v not found.",
			fnMeta.Labels["app.kubernetes.io/instance"],
		)
	}
	if cmName == "" {
		return nil, fmt.Errorf(
			"configmap instance %v not found.",
			fnMeta.Labels["app.kubernetes.io/instance"],
		)
	}

	return &templateData{
		ObjectMeta:      &fnMeta.ObjectMeta,
		StatefulSetName: stsName,
		ConfigMapName:   cmName,
	}, nil
}
