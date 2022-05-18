---
apiVersion: batch/v1
kind: Job
metadata:
  generateName: {{ .GenerateName }}
  namespace: {{ .Namespace }}
  labels:
    {{- range $key, $value := .Labels }}
    {{ $key }}: "{{ $value }}"
    {{- end }}
spec:
  backoffLimit: 3
  completions: 1
  parallelism: 1
  template:
    spec:
      restartPolicy: OnFailure
      serviceAccountName: {{ .ServiceAccount }}
      securityContext:
        runAsUser: 65534
        runAsGroup: 65534
        fsGroup: 65534
      volumes:
        # Used to hold the terraform module source code
        - name: source
          emptyDir: {}
        # Used to hold runner scripts and shared state
        - name: run
          emptyDir: {}
        # These contains auto generated configuation required for the job
        - name: config
          secret:
            secretName: {{ .Secrets.Config }}
            optional: false
            items:
              - key: backend.tf
                path: backend.tf
              - key: provider.tf
                path: provider.tf
              {{- if .EnableVariables }}
              - key: variables.tfvars.json
                path: variables.tfvars.json
              {{- end }}

      initContainers:
        - name: setup
          image: {{ .Images.Executor }}
          imagePullPolicy: {{ .ImagePullPolicy }}
          command:
            - /bin/step
          args:
            - --command=/bin/mkdir -p /run/bin
            - --command=/bin/mkdir -p /run/steps
            - --command=/bin/cp /run/config/* /data
            - --command=/bin/cp /bin/step /run/bin/step
            - --command=/bin/cp /bin/kubectl /run/bin/kubectl
            - --command=/bin/source --dest=/data --source={{ .Configuration.Module }}
          {{- if .Secrets.Config }}
          envFrom:
            - secretRef:
                name: {{ .Secrets.Config }}
                optional: false
          {{- end }}
          volumeMounts:
            - name: config
              mountPath: /run/config
              reaonly: true
            - name: run
              mountPath: /run
            - name: source
              mountPath: /data

        - name: init
          image: {{ .Images.Terraform }}
          workingDir: /data
          command:
            - /bin/terraform
          args:
            - init
          securityContext:
            capabilities:
              drop: [ALL]
          volumeMounts:
            - name: source
              mountPath: /data

      containers:
      - name: terraform
        image: {{ .Images.Terraform }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        workingDir: /data
        command:
          - /run/bin/step
        args:
          {{- if eq .Stage "plan" }}
          - --command=/bin/terraform plan {{ .TerraformArguments }} -out=/run/plan.out -lock=true
          - --command=/bin/terraform show -json /run/plan.out > /run/plan.json
          {{- end }}
          {{- if eq .Stage "apply" }}
          - --command=/bin/terraform apply {{ .TerraformArguments }} -auto-approve -lock=true
          {{- end }}
          {{- if eq .Stage "destroy" }}
          - --command=/bin/terraform destroy {{ .TerraformArguments }} -auto-approve
          {{- end }}
          - --on-error=/run/steps/terraform.failed
          - --on-success=/run/steps/terraform.complete
        env:
          - name: CONFIGURATION_NAME
            value: {{ .Configuration.Name }}
          - name: CONFIGURATION_NAMESPACE
            value: {{ .Configuration.Namespace }}
          - name: CONFIGURATION_UUID
            value: "{{ .Configuration.UUID }}"
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        {{- if eq .Provider.Source "secret" }}
        envFrom:
          - secretRef:
              name: {{ .Provider.SecretRef.Name }}
        {{- end }}
        resources:
          limits:
            cpu: 1
            memory: 1Gi
          requests:
            cpu: 5m
            memory: 32Mi
        securityContext:
          capabilities:
            drop: [ALL]
        volumeMounts:
          - name: run
            mountPath: /run
          - name: source
            mountPath: /data

      {{- if and (.EnableInfraCosts) (eq .Stage "plan") }}
      - name: costs
        image: {{ .Images.Infracosts }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        command:
          - /run/bin/step
        args:
          - --command=/usr/bin/infracost breakdown --path /run/plan.json
          - --command=/usr/bin/infracost breakdown --path /run/plan.json --format json > /run/costs.json
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) delete secret $(COST_REPORT_NAME) --ignore-not-found
          - --command=/run/bin/kubectl -n $(KUBE_NAMESPACE) create secret generic $(COST_REPORT_NAME) --from-file=/run/costs.json
          - --is-failure=/run/steps/terraform.failed
          - --timeout=5m
          - --wait-on=/run/steps/terraform.complete
        env:
          - name: COST_REPORT_NAME
            value: {{ .Secrets.InfracostsReport }}
          - name: KUBE_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        {{- if .Secrets.Infracosts }}
        envFrom:
          - secretRef:
              name: {{ .Secrets.Infracosts }}
        {{- end }}
        securityContext:
          capabilities:
            drop: [ALL]
        volumeMounts:
          - name: run
            mountPath: /run
          - name: source
            mountPath: /data
      {{- end }}

      {{- if and (.EnablePolicy) (eq .Stage "plan") }}
      - name: verify-policy
        image: {{ .Images.Policy }}
        imagePullPolicy: {{ .ImagePullPolicy }}
        workingDir: /data
        command:
          - /run/bin/step
        args:
          - --command=/usr/local/bin/checkov --framework terraform_plan -f /run/plan.json
          - --is-failure=/run/steps/terraform.failed
          - --wait-on=/run/steps/terraform.complete
        {{- if .Secrets.Policy }}
        envFrom
          - secretRef:
              name: {{ .Secrets.Policy }}
              optional: false
        {{- end }}
        securityContext:
          capabilities:
            drop: [ALL]
        volumeMounts:
          - name: run
            mountPath: /run
          - name: source
            mountPath: /data
      {{- end }}
