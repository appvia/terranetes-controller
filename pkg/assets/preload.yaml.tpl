---
apiVersion: batch/v1
kind: Job
metadata:
  generateName: {{ .GenerateName }}
  namespace: {{ .Controller.Namespace }}
  labels:
    {{- range $key, $value := .Labels }}
    {{ $key }}: "{{ $value }}"
    {{- end }}
spec:
  backoffLimit: 1
  completions: 1
  parallelism: 1
  # retain the jobs for 6 hours
  ttlSecondsAfterFinished: 28800
  template:
    metadata:
      labels:
        {{- range $key, $value := .Labels }}
        {{ $key }}: "{{ $value }}"
        {{- end }}
        aadpodidbinding: terranetes-executor
    spec:
      # https://github.com/kubernetes/kubernetes/issues/74848
      restartPolicy: Never
      {{- if eq .Provider.Source "injected" }}
      serviceAccountName: {{ .Provider.ServiceAccount }}
      {{- else }}
      serviceAccountName: {{ .ServiceAccount }}
      {{- end }}
      securityContext:
        runAsUser: 65534
        runAsGroup: 65534
        fsGroup: 65534
      containers:
      - name: loader
        image: {{ .ContainerImage }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        securityContext:
          capabilities:
            drop: [ALL]
        command:
          - /bin/preload
        env:
          - name: CLOUD
            value: {{ .Provider.Cloud }}
          - name: CLUSTER
            value: {{ .Cluster }}
          - name: CONTEXT
            value: {{ .Context.Name }}
          - name: PROVIDER
            value: {{ .Provider.Name }}
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: REGION
            value: {{ .Region }}
        envFrom:
        {{- if eq .Provider.Source "secret" }}
          - secretRef:
              name: {{ .Provider.SecretRef.Name }}
        {{- end }}
        resources:
          limits:
            cpu: 5m
            memory: 128Mi
          requests:
            cpu: 5m
            memory: 32Mi
