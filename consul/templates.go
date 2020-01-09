package main

func (f *filter) defaultTemplates() map[string]string {
	return map[string]string{
		"cm":      cmTemplate,
		"sts":     stsTemplate,
		"svc":     svcTemplate,
		"dns-svc": dnsSvcTemplate,
		"ui-svc":  uiSvcTemplate,
	}
}

var cmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}
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

var stsTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}
spec:
  serviceName: {{ .Name }}
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
            - -retry-join={{ .Name }}.$(NAMESPACE).svc.cluster.local
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
            - name: consul-data
              mountPath: /consul/data
            - name: consul-configs
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
        - name: consul-data
          emptyDir: {}
        - name: consul-configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}
`

var svcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
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

var dnsSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-dns
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

var uiSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-ui
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8500
`
