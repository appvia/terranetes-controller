Name:         {{ .Object.Name }}
Namespace:    {{ .Object.Namespace }}
Created:      {{ .Object.CreationTimestamp }}
Status:       {{ default "Unknown" .Object.Status.ResourceStatus }}
{{- if .Object.Annotations }}
Annotations:
{{- range $key, $value := .Object.Annotations }}
{{- printf "%-28s %-20s" $key $value | nindent 14 }}
{{- end }}
{{- else }}
Annotations:  None
{{- end }}
{{- if .Object.Labels }}
Labels:
{{- range $key, $value := .Object.Labels }}
{{- printf "%-28s %-20s" $key $value | nindent 14 }}
{{- end }}
{{- else }}
Labels:       None
{{- end }}

Conditions:
==========
{{- if .Object.Status.Conditions }}
{{ printf "%-18s %-18s %s" "Name" "Reason" "Message" }}
{{- range $condition := .Object.Status.Conditions }}
{{ printf "%-18s %-18s %s" .Name .Reason (default "" .Message) }}
{{- end }}
{{- else }}
 None
{{ end }}

Configuration:
=============
{{- if .Object.Spec.Auth }}
Authentication: {{ .Object.Namespace }}/{{ .Object.Spec.Auth.Name }}
{{- else }}
Authentication: None
{{- end }}
Module:         {{ .Object.Spec.Module }}
Provider:       {{ .Object.Spec.ProviderRef.Name }}
{{- if .Object.Spec.WriteConnectionSecretToRef }}
Secret:         {{ .Object.Namespace }}/{{ .Object.Spec.WriteConnectionSecretToRef.Name }}
{{- else }}
Secret:         None
{{- end }}

{{- if .Policy }}

Checkov Security Policy:
=======================
{{- if .Policy.results }}
Status:        Configuration has passed {{ .Policy.results.passed_checks | len }} and failed on {{ .Policy.results.failed_checks | len }} checks.
{{ range $check := .Policy.results.failed_checks }}
{{ printf "%-15s%s" $check.check_id "FAILED" }}
├─ Name:       {{ $check.check_name }}
├─ Resource:   {{ $check.resource_address }}
└─ Guide:      {{ default "-" $check.guideline }}
{{- end }}
{{- if .EnablePassedPolicy }}
{{- range $check := .Policy.results.passed_checks }}
{{ printf "%-15s%s" $check.check_id "PASSED" }}
├─ Name:       {{ $check.check_name }}
├─ Resource:   {{ $check.resource_address }}
└─ Guide:      {{ default "-" $check.guideline }}
{{- end }}
{{- end }}
{{- else }}
Status:        No matching security checks found, configuration passed.
{{- end }}
{{- if .Cost }}

Predicted Costs:
===============
{{- if ge (.Cost.breakdown.resources | len) 1 }}
{{- range $index, $resource := .Cost.breakdown.resources }}
{{- if $index }}
├─ {{ printf "%-32s $%-5s $%s" $resource.name (default "0.00" $resource.monthlyCost) (default "0.00" $resource.hourlyCost) }}
{{- else }}
└─ {{ printf "%-32s $%-5s $%s" $resource.name (default "0.00" $resource.monthlyCost) ((default "0.00" $resource.hourlyCost) | substr 0 5) }}
{{- end }}
{{- end }}

Monthly Total: ${{ .Cost.breakdown.totalMonthlyCost }}
Hourly  Total: ${{ .Cost.breakdown.totalHourlyCost | substr 0 5 }}
{{- end }}

{{- end }}
{{- end }}
