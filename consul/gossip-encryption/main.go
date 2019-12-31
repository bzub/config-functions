package main

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

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
	if err := f.VerifyMeta("consul-gossip-encryption"); err != nil {
		return nil, err
	}

	// Get resources we care about from input.
	managedRs, err := f.ManagedResources(in)
	if err != nil {
		return nil, err
	}

	// Add gossip encryption config secret volume to Consul server
	// StatefulSet.
	if err := f.injectSecretVolume(managedRs); err != nil {
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
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	// Find the name of the Consul server StatefulSet to patch.
	var stsName string
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		switch rMeta.Kind {
		case "StatefulSet":
			stsName = rMeta.Name
		}
	}
	if stsName == "" {
		return nil, fmt.Errorf(
			"statefulset instance %v not found.",
			fnMeta.Labels["app.kubernetes.io/instance"],
		)
	}

	return &templateData{
		ObjectMeta:      &fnMeta.ObjectMeta,
		StatefulSetName: stsName,
	}, nil
}

// injectSecretVolume adds the gossip encryption Consul config secret as a
// projected volume source to the Consul server Resource config.
func (f *filter) injectSecretVolume(in []*yaml.RNode) error {
	// Find the StatefulSet
	var sts *yaml.RNode
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return err
		}

		if rMeta.Kind == "StatefulSet" {
			sts = r
			break
		}
	}
	if sts == nil {
		return fmt.Errorf("StatefulSet not found.")
	}

	// Get data for templates.
	data, err := f.templateData(in)
	if err != nil {
		return err
	}

	// Execute the projected volume source template.
	buff := &bytes.Buffer{}
	t := template.Must(template.New("pv-source").Parse(projectedVolumeSourceTemplate))
	if err := t.Execute(buff, data); err != nil {
		return err
	}
	vol, err := yaml.Parse(buff.String())
	if err != nil {
		return err
	}

	// Add secret volume to StatefulSet.
	err = sts.PipeE(
		yaml.Lookup(
			"spec", "template", "spec", "volumes",
			"[name=configs]", "projected", "sources",
		),
		yaml.Append(vol.YNode().Content...),
	)
	if err != nil {
		return nil
	}

	return nil
}
