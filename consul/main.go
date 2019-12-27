package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
			&filters.FileSetter{
				FilenamePattern: filepath.Join("resources", "%n_%k.yaml"),
			},
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
	Metadata struct {
		// Name is used for Resource metadata.name either directly or
		// as a prefix.
		Name string `yaml:"name"`
	} `yaml:"metadata"`

	Spec struct {
		// Replicas is the number of StatefulSet replicas.
		// Defaults to the REPLICAS env var, or 1
		Replicas *int `yaml:"replicas"`
	} `yaml:"spec"`
}

// filter implements kio.Filter
type filter struct {
	rw *kio.ByteReadWriter
}

// Filter generates Resources.
func (f *filter) Filter(in []*yaml.RNode) ([]*yaml.RNode, error) {
	api := f.parseAPI()

	// execute the configmap template
	buff := &bytes.Buffer{}
	t := template.Must(template.New("consul-cm").Parse(configMapTemplate))
	if err := t.Execute(buff, api); err != nil {
		return nil, err
	}
	cm, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}

	// execute the statefulset template
	buff = &bytes.Buffer{}
	t = template.Must(template.New("consul-sts").Parse(statefulSetTemplate))
	if err := t.Execute(buff, api); err != nil {
		return nil, err
	}
	sts, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}

	// execute the service template
	buff = &bytes.Buffer{}
	t = template.Must(template.New("consul-svc").Parse(serviceTemplate))
	if err := t.Execute(buff, api); err != nil {
		return nil, err
	}
	svc, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}

	// execute the dns service template
	buff = &bytes.Buffer{}
	t = template.Must(template.New("consul-svc-dns").Parse(dnsServiceTemplate))
	if err := t.Execute(buff, api); err != nil {
		return nil, err
	}
	dns, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}

	// execute the ui service template
	buff = &bytes.Buffer{}
	t = template.Must(template.New("consul-svc-ui").Parse(uiServiceTemplate))
	if err := t.Execute(buff, api); err != nil {
		return nil, err
	}
	ui, err := yaml.Parse(buff.String())
	if err != nil {
		return nil, err
	}

	in = append(in, cm, sts, svc, dns, ui)
	return in, nil
}

// parseAPI parses the functionConfig into an API struct, and validates the
// input.
func (f *filter) parseAPI() API {
	// parse the input function config
	var api API
	if err := yaml.Unmarshal([]byte(f.rw.FunctionConfig.MustString()), &api); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Default functionConfig values from environment variables if they are
	// not set in the functionConfig
	r := os.Getenv("REPLICAS")
	if r != "" && api.Spec.Replicas == nil {
		replicas, err := strconv.Atoi(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		api.Spec.Replicas = &replicas
	}
	if api.Spec.Replicas == nil {
		r := 1
		api.Spec.Replicas = &r
	}
	if api.Metadata.Name == "" {
		fmt.Fprintf(os.Stderr, "must specify metadata.name\n")
		os.Exit(1)
	}

	return api
}

var configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
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
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: consul
      app.kubernetes.io/instance: {{ .Metadata.Name }}
  serviceName: {{ .Metadata.Name }}
  podManagementPolicy: Parallel
  updateStrategy:
    type: RollingUpdate
  replicas: {{ .Spec.Replicas }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: consul
        app.kubernetes.io/instance: {{ .Metadata.Name }}
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
            - -retry-join={{ .Metadata.Name }}.$(NAMESPACE).svc.cluster.local
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
              value: "{{ .Spec.Replicas }}"
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
            name: {{ .Metadata.Name }}
`

var serviceTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Metadata.Name }}
  labels:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
spec:
  clusterIP: None
  publishNotReadyAddresses: true
  selector:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
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
  name: {{ .Metadata.Name }}-dns
  labels:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
spec:
  selector:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
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
  name: {{ .Metadata.Name }}-ui
  labels:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
spec:
  selector:
    app.kubernetes.io/name: consul
    app.kubernetes.io/instance: {{ .Metadata.Name }}
  ports:
    - name: http
      port: 80
      targetPort: 8500
`
