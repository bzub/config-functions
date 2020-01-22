package vault

func (f *VaultFilter) serverTemplates() map[string]string {
	return map[string]string{
		"server-cm":  serverCmTemplate,
		"server-sts": serverStsTemplate,
		"server-svc": serverSvcTemplate,
	}
}

var serverCmTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
data:
  00-defaults.hcl: |-
    listener "tcp" {
      tls_disable = 1
      address = "[::]:8200"
      cluster_address = "[::]:8201"
    }
  storage-backend.hcl: |-
    storage "file" {
      path = "/vault/data"
    }
`

var serverStsTemplate = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  serviceName: {{ .Name }}-server
  podManagementPolicy: Parallel
  updateStrategy:
    type: RollingUpdate
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
      app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
        app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
    spec:
      containers:
        - name: vault-server
          image: docker.io/library/vault:1.3.1
          command:
            - /usr/local/bin/docker-entrypoint.sh
            - vault
            - server
            - -config=/vault/configs
          envFrom:
            - configMapRef:
                name: {{ .Name }}
          env:
            - name: VAULT_ADDR
              value: http://127.0.0.1:8200
          lifecycle:
            preStop:
              exec:
                command:
                  - vault
                  - step-down
          ports:
            - name: http
              containerPort: 8200
            - name: internal
              containerPort: 8201
            - name: replication
              containerPort: 8202
          readinessProbe:
            exec:
              command:
                - /bin/sh
                - -ec
                - vault status
            failureThreshold: 2
            initialDelaySeconds: 5
            periodSeconds: 3
            successThreshold: 1
            timeoutSeconds: 5
          securityContext:
            capabilities:
              add:
                - IPC_LOCK
          volumeMounts:
            - name: vault-configs
              mountPath: /vault/configs
              readOnly: true
      volumes:
        - name: vault-configs
          projected:
            sources:
              - configMap:
                  name: {{ .Name }}-server
`

var serverSvcTemplate = `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-server
  namespace: "{{ .Namespace }}"
  labels:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
spec:
  selector:
    app.kubernetes.io/name: {{ index .Labels "app.kubernetes.io/name" }}
    app.kubernetes.io/instance: {{ index .Labels "app.kubernetes.io/instance" }}
  publishNotReadyAddresses: true
  ports:
    - name: http
      port: 8200
      targetPort: 8200
    - name: internal
      port: 8201
      targetPort: 8201
`
