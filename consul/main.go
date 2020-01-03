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

var projectedVolumeSecretTemplate = `
- secret:
    name: {{ .Name }}-gossip-encryption
`

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
	if err := f.VerifyMeta("consul-server"); err != nil {
		return nil, err
	}

	// Get resources we care about from input.
	managedRs, err := f.ManagedResources(in)
	if err != nil {
		return nil, err
	}

	// Find the Consul StatefulSet.
	sts, err := getConsulStatefulSet(managedRs)
	if err != nil {
		return nil, err
	}

	// Get data for templates.
	data, err := f.templateData(sts)
	if err != nil {
		return nil, err
	}

	// Generate Consul server Resources from templates.
	templateRs, err := cfunc.ParseTemplates(f.defaultTemplates(), data)
	if err != nil {
		return nil, err
	}

	if f.gossipEnabled() {
		// Use templateRs StatefulSet.
		sts, err = getConsulStatefulSet(templateRs)
		if err != nil {
			return nil, err
		}

		// Add gossip encryption config secret volume to Consul server
		// StatefulSet.
		if err := f.injectGossipSecretVolume(sts); err != nil {
			return nil, err
		}

		// Generate gossip Resouces from templates.
		gossipRs, err := cfunc.ParseTemplates(f.gossipTemplates(), data)
		if err != nil {
			return nil, err
		}

		templateRs = append(templateRs, gossipRs...)
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
func (f *filter) templateData(sts *yaml.RNode) (*templateData, error) {
	fnMeta, err := f.RW.FunctionConfig.GetMeta()
	if err != nil {
		return nil, err
	}

	// Defaults
	data := &templateData{
		ObjectMeta:      &fnMeta.ObjectMeta,
		Replicas:        1,
		StatefulSetName: fnMeta.Name,
	}

	if sts == nil {
		// Use defaults if no StatefulSet is provided (needs to be
		// generated).
		return data, nil
	}

	stsMeta, err := sts.GetMeta()
	if err != nil {
		return nil, err
	}

	// Use the provided StatefulSet name for template data.
	data.StatefulSetName = stsMeta.Name

	// Find the number of replicas for the workload being managed. Defaults
	// to 1 if replicas is omitted in the input Resource config.
	value, err := sts.Pipe(yaml.Lookup("spec", "replicas"))
	if err != nil {
		return nil, err
	}
	if value != nil {
		data.Replicas, err = strconv.Atoi(value.YNode().Value)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// injectGossipSecretVolume adds the gossip encryption Consul config secret as
// a projected volume source to the Consul server Resource config.
//
// TODO: Strategic merge patch seems to replace rather than merge projected
// volume sources. This should be in the patch variable and this function
// removed once that's fixed.
func (f *filter) injectGossipSecretVolume(sts *yaml.RNode) error {
	// Get data for templates.
	data, err := f.templateData(sts)
	if err != nil {
		return err
	}

	// Execute the projected volume source template.
	buff := &bytes.Buffer{}
	t := template.Must(template.New("gossip-pvs").Parse(projectedVolumeSecretTemplate))
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

func getConsulStatefulSet(in []*yaml.RNode) (*yaml.RNode, error) {
	// Find the StatefulSet
	var sts *yaml.RNode
	for _, r := range in {
		rMeta, err := r.GetMeta()
		if err != nil {
			return nil, err
		}

		if rMeta.Kind == "StatefulSet" {
			sts = r
			break
		}
	}
	if sts == nil {
		// return nil, fmt.Errorf("StatefulSet not found.")
		return nil, nil
	}

	return sts, nil
}

func (f *filter) gossipEnabled() bool {
	enabled, _ := f.RW.FunctionConfig.Pipe(
		yaml.Lookup("spec", "gossipEncryption", "enabled"),
	)
	if enabled.Document().Value == "true" {
		return true
	}

	return false
}
