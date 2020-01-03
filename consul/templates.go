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

func (f *filter) gossipTemplates() map[string]string {
	return map[string]string{
		"gossip-job":         gossipJobTemplate,
		"gossip-sa":          gossipSATemplate,
		"gossip-role":        gossipRoleTemplate,
		"gossip-rolebinding": gossipRoleBindingTemplate,
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
            - name: data
              mountPath: /consul/data
            - name: configs
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
        - name: configs
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

var gossipJobTemplate = `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Name }}-gossip-encryption
spec:
  template:
    spec:
      serviceAccountName: {{ .Name }}-gossip-encryption
      restartPolicy: OnFailure
      initContainers:
        - name: generate-gossip-encryption-config
          image: docker.io/library/consul:1.6.2
          command:
            - /bin/sh
            - -ec
            - |-
              config_file=/config/generated/01-gossip-encryption.json
              cat <<EOF > "${config_file}"
              {
                "encrypt": "$(consul keygen)",
                "encrypt_verify_incoming": true,
                "encrypt_verify_outgoing": true
              }
          volumeMounts:
            - mountPath: /config/generated
              name: config-generated
      containers:
        - name: create-gossip-encryption-config-secret
          image: k8s.gcr.io/hyperkube:v1.17.0
          command:
            - /bin/sh
            - -ec
            - |-
              secret="{{ .Name }}-gossip-encryption"
              config_dir="/config/generated"

              if [ ! "$(kubectl get secret --no-headers "${secret}"|wc -l)" = "0" ]; then
                echo "[INFO] \"secret/${secret}\" already exists. Exiting."
                exit 0
              fi

              echo "[INFO] Creating \"secret/${secret}\"."
              kubectl create secret generic "--from-file=${config_dir}" "${secret}"
          volumeMounts:
            - mountPath: /config/generated
              name: config-generated
      volumes:
        - name: config-generated
          emptyDir: {}
`

var gossipSATemplate = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}-gossip-encryption
`

var gossipRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .Name }}-gossip-encryption
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
      - create
`

var gossipRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .Name }}-gossip-encryption
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .Name }}-gossip-encryption
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}-gossip-encryption
`
