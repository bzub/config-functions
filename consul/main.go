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
)

func main() {
	rw := &kio.ByteReadWriter{
		Reader:                os.Stdin,
		Writer:                os.Stdout,
		KeepReaderAnnotations: true,
	}

	err := kio.Pipeline{
		Inputs: []kio.Reader{rw},
		Filters: []kio.Filter{
			&filter{rw: rw},
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

// API is the function configuration spec.
type API struct {
	Metadata *yaml.ObjectMeta
}

// filter implements kio.Filter
type filter struct {
	rw        *kio.ByteReadWriter
	api       *API
	inputs    []*yaml.RNode
	templates map[string]string
}

// Filter generates Resources.
func (f *filter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	f.inputs = in

	if err := f.init(); err != nil {
		return nil, err
	}

	outputs, err := f.outputs()
	if err != nil {
		return nil, err
	}

	return append(in, outputs...), nil
}

// init parses the functionConfig into an API struct.
func (f *filter) init() error {
	if f.api == nil {
		f.api = &API{}
	}

	// Parse the function config and set defaults.
	if err := yaml.Unmarshal([]byte(f.rw.FunctionConfig.MustString()), f.api); err != nil {
		return err
	}
	if f.api.Metadata.Name == "" {
		return fmt.Errorf("must specify metadata.name: \n%v\n",
			f.rw.FunctionConfig.MustString())
	}
	if f.api.Metadata.Labels == nil {
		f.api.Metadata.Labels = map[string]string{}
	}
	f.api.Metadata.Labels["app.kubernetes.io/name"] = "consul-server"
	f.api.Metadata.Labels["app.kubernetes.io/instance"] = f.api.Metadata.Name

	return nil
}

func (f *filter) outputs() ([]*yaml.RNode, error) {
	if len(f.templates) == 0 {
		f.templates = f.defaultTemplates()
	}

	outputs := []*yaml.RNode{}
	for name, tmpl := range f.templates {
		outR, err := f.parseNewTemplate(name, tmpl)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, outR)
	}

	outputs, err := f.populateMeta(outputs...)
	if err != nil {
		return nil, err
	}

	return outputs, nil
}

func (f *filter) parseNewTemplate(name, tmpl string) (*yaml.RNode, error) {
	buff := &bytes.Buffer{}
	t := template.Must(template.New(name).Parse(tmpl))
	if err := t.Execute(buff, f); err != nil {
		return nil, err
	}
	r, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *filter) populateMeta(resources ...*yaml.RNode) ([]*yaml.RNode, error) {
	for _, r := range resources {
		rLabels, err := r.Pipe(yaml.LookupCreate(
			yaml.MappingNode,
			"metadata", "labels",
		))
		if err != nil {
			return nil, err
		}

		for k, v := range f.api.objectMeta().Labels {
			err = rLabels.PipeE(
				yaml.SetField(k, yaml.NewScalarRNode(v)),
			)
			if err != nil {
				return nil, err
			}
		}
	}

	return resources, nil
}

func (f *filter) defaultTemplates() map[string]string {
	return map[string]string{
		"configmap":   configMapTemplate,
		"statefulset": statefulSetTemplate,
		"service":     serviceTemplate,
		"dns-service": dnsServiceTemplate,
		"ui-service":  uiServiceTemplate,
	}
}

func (a *API) objectMeta() *yaml.ObjectMeta {
	return &yaml.ObjectMeta{
		Name:   a.Metadata.Name,
		Labels: a.Metadata.Labels,
	}
}

func (f *filter) API() *API {
	return f.api
}

func (f *filter) Replicas() int {
	replicas := 1

	for _, r := range f.inputs {
		meta, err := r.GetMeta()
		if err != nil {
			continue
		}
		if meta.Kind == "StatefulSet" && meta.Name == f.api.Metadata.Name {
			value, err := r.Pipe(yaml.Lookup("spec", "replicas"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			if value == nil {
				continue
			}

			replicas, err = strconv.Atoi(value.YNode().Value)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
	}

	return replicas
}

var configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .API.Metadata.Name }}
data:
  00-defaults.hcl: |-
    acl = {
      enabled = true
      default_policy = "allow"
      enable_token_persistence = true
    }

    connect = {
      enabled = true
    }
`

var statefulSetTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .API.Metadata.Name }}
spec:
  serviceName: {{ .API.Metadata.Name }}
  podManagementPolicy: Parallel
  updateStrategy:
    type: RollingUpdate
  template:
    spec:
      terminationGracePeriodSeconds: 10
      securityContext:
        fsGroup: 1000
      containers:
        - name: consul
          image: docker.io/library/consul:1.6.2
          command:
            - consul
            - agent
            - -advertise=$(POD_IP)
            - -bind=0.0.0.0
            - -bootstrap-expect=$(CONSUL_REPLICAS)
            - -client=0.0.0.0
            - -config-dir=/consul/config
            - -data-dir=/consul/data
            - -ui
            - -retry-join={{ .API.Metadata.Name }}.$(NAMESPACE).svc.cluster.local
            - -server
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: CONSUL_REPLICAS
              value: "{{ .Replicas }}"
          volumeMounts:
            - name: data
              mountPath: /consul/data
            - name: config
              mountPath: /consul/config
          lifecycle:
            preStop:
              exec:
                command:
                - /bin/sh
                - -c
                - consul leave
          ports:
            - containerPort: 8500
              name: http
              protocol: "TCP"
            - containerPort: 8301
              name: serflan-tcp
              protocol: "TCP"
            - containerPort: 8301
              name: serflan-udp
              protocol: "UDP"
            - containerPort: 8302
              name: serfwan-tcp
              protocol: "TCP"
            - containerPort: 8302
              name: serfwan-udp
              protocol: "UDP"
            - containerPort: 8300
              name: server
              protocol: "TCP"
            - containerPort: 8600
              name: dns-tcp
              protocol: "TCP"
            - containerPort: 8600
              name: dns-udp
              protocol: "UDP"
          readinessProbe:
            exec:
              command:
                - "/bin/sh"
                - "-ec"
                - |
                  curl http://127.0.0.1:8500/v1/status/leader 2>/dev/null | \
                  grep -E '".+"'
            failureThreshold: 2
            initialDelaySeconds: 5
            periodSeconds: 3
            successThreshold: 1
            timeoutSeconds: 5
      volumes:
        - name: data
          emptyDir: {}
        - name: config
          configMap:
            name: {{ .API.Metadata.Name }}
`

var serviceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .API.Metadata.Name }}
spec:
  clusterIP: None
  publishNotReadyAddresses: true
  ports:
    - name: http
      port: 8500
      targetPort: http
    - name: serflan-tcp
      protocol: "TCP"
      port: 8301
      targetPort: serflan-tcp
    - name: serflan-udp
      protocol: "UDP"
      port: 8301
      targetPort: serflan-udp
    - name: serfwan-tcp
      protocol: "TCP"
      port: 8302
      targetPort: serfwan-tcp
    - name: serfwan-udp
      protocol: "UDP"
      port: 8302
      targetPort: serfwan-udp
    - name: server
      port: 8300
      targetPort: server
    - name: dns-tcp
      protocol: "TCP"
      port: 8600
      targetPort: dns-tcp
    - name: dns-udp
      protocol: "UDP"
      port: 8600
      targetPort: dns-udp
`

var dnsServiceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .API.Metadata.Name }}-dns
spec:
  ports:
    - name: dns-tcp
      port: 53
      protocol: "TCP"
      targetPort: dns-tcp
    - name: dns-udp
      port: 53
      protocol: "UDP"
      targetPort: dns-udp
`

var uiServiceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .API.Metadata.Name }}-ui
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8500
`
