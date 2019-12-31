package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"text/template"

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

	// Replicas is the number of configured Consul server replicas. Used
	// for other options like --bootstrap-expect.
	Replicas int
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
	if err := f.VerifyMeta("consul-server"); err != nil {
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
	templateRs := []*yaml.RNode{}
	for name, tmpl := range f.defaultTemplates() {
		buff := &bytes.Buffer{}
		t := template.Must(template.New(name).Parse(tmpl))
		if err := t.Execute(buff, data); err != nil {
			return nil, err
		}
		r, err := yaml.Parse(buff.String())
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, r)
	}

	// Set function config metadata on generated Resources.
	for _, r := range templateRs {
		// Set labels from config function to resources.
		if err := f.SetLabels(r); err != nil {
			return nil, err
		}

		// Set namespace from config function to resources.
		if err := f.SetNamespace(r); err != nil {
			return nil, err
		}
	}

	// Merge our templated Resources into the input Resources.
	return append(in, templateRs...), nil
}

// templateData populates a struct with information needed for Resource
// templates.
func (f *filter) templateData(in []*yaml.RNode) (*templateData, error) {
	// Find the number of replicas for the workload being managed. Defaults
	// to 1 if replicas is omitted in the input Resource config.
	replicas := 1
	for _, r := range in {
		meta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}
		if meta.Kind != "StatefulSet" {
			continue
		}

		value, err := r.Pipe(yaml.Lookup("spec", "replicas"))
		if err != nil {
			return nil, err
		}
		if value == nil {
			break
		}

		replicas, err = strconv.Atoi(value.YNode().Value)
		if err != nil {
			return nil, err
		}
	}

	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	return &templateData{
		ObjectMeta: &fnMeta.ObjectMeta,
		Replicas:   replicas,
	}, nil
}
