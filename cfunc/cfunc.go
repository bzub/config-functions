package cfunc

import (
	"bytes"
	"text/template"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func FixStyles(in ...*yaml.RNode) error {
	for _, r := range in {
		r.YNode().Style = 0
		switch r.YNode().Kind {
		case yaml.MappingNode:
			err := r.VisitFields(func(node *yaml.MapNode) error {
				return FixStyles(node.Value)
			})
			if err != nil {
				return err
			}
		case yaml.SequenceNode:
			err := r.VisitElements(func(node *yaml.RNode) error {
				return FixStyles(node)
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ParseTemplates(tmpls map[string]string, data interface{}) ([]*yaml.RNode, error) {
	templateRs := []*yaml.RNode{}
	for name, tmpl := range tmpls {
		r, err := ParseTemplate(name, tmpl, data)
		if err != nil {
			return nil, err
		}
		templateRs = append(templateRs, r)
	}

	return templateRs, nil
}

func ParseTemplate(name, tmpl string, data interface{}) (*yaml.RNode, error) {
	buff := &bytes.Buffer{}
	t := template.Must(template.New(name).Parse(tmpl))
	if err := t.Execute(buff, data); err != nil {
		return nil, err
	}
	r, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}
	return r, nil
}
