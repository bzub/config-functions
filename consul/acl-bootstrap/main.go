package main

import (
	"fmt"
	"os"

	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/bzub/config-functions/cfunc"
)

// filter implements kio.Filter
type filter struct {
	*cfunc.CFunc
}

// templateData holds information used in Resource templates.
type templateData struct {
	// ObjectMeta contains Resource metadata to use in templates.
	*yaml.ObjectMeta

	// ExecArg is the "(POD | TYPE/NAME)" argument for kubectl exec so we
	// can run commands on a certain Consul cluster.
	ExecArg string
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
	if err := f.VerifyMeta("consul-acl-bootstrap"); err != nil {
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

	// Find the name of the Consul StatefulSet to exec into.
	stsName := ""
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		if rMeta.Kind == "StatefulSet" {
			stsName = rMeta.Name
			break
		}
	}
	if stsName == "" {
		return nil, fmt.Errorf(
			"statefulset instance %v not found.",
			fnMeta.Labels["app.kubernetes.io/instance"],
		)
	}

	return &templateData{
		ObjectMeta: &fnMeta.ObjectMeta,
		ExecArg:    "sts/" + stsName,
	}, nil
}
