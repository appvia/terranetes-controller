Name:         {{ .Object.metadata.name }}
Namespace:    {{ .Object.metadata.namespace }}
Created:      {{ .Object.metadata.creationTimestamp }}
Status:       {{ default "Unknown" .Object.status.resourceStatus }}
{{- if .Object.metadata.annotations }}
Annotations:  {{ range $key, $value := .Object.metadata.annotations }}{{ $key }}: {{ $value }}{{- end }}
{{- else }}
Annotations:  None
{{- end }}
{{- if .Object.metadata.labels }}
Labels:       {{ range $key, $value := .Object.metadata.labels }}{{ $key }}: {{ $value }}{{- end }}
{{- else }}
Labels:       None
{{- end }}

Conditions:
==========
{{- if .Object.status.conditions }}
{{ printf "%-18s %-18s %s" "Name" "Reason" "Message" }}
{{- range $condition := .Object.status.conditions }}
{{ printf "%-18s %-18s %s" .name .reason (default "" .message) }}
{{- end }}
{{- else }}
 None
{{ end }}

Configuration:
=============
{{- if .Object.spec.auth }}
Authentication: {{ .Object.metadata.namespace }}/{{ .Object.spec.auth.name }}
{{- else }}
Authentication: None
{{- end }}
Module:         {{ .Object.spec.module }}
Provider:       {{ .Object.spec.providerRef.name }}
{{- if .Object.spec.writeConnectionSecretToRef }}
Secret:         {{ .Object.metadata.namespace }}/{{ .Object.spec.writeConnectionSecretToRef.name }}
{{- else }}
Secret:         None
{{- end }}
{{- if and (.Object.status.costs) (.Object.status.costs.enabled) }}

{{- if .Policy }}

Checkov Security Policy:
=======================
Status:         Configuration has passed {{ .Policy.results.passed_checks | len }} and failed on {{ .Policy.results.failed_checks | len }} checks.
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
{{- end }}

{{- if .Cost }}

Predicted Costs:
===============
{{- if ge (.Cost.breakdown.resources | len) 1 }}
{{ printf "%-36s%-14s%s" "Resources:" "Monthly" "Hourly" }}
{{- range $index, $resource := .Cost.breakdown.resources }}
{{- if $index }}
├─ {{ printf "%-32s $%-12s $%s" $resource.name $resource.monthlyCost $resource.hourlyCost }}
{{- else }}
└─ {{ printf "%-32s $%-12s $%s" $resource.name $resource.monthlyCost ($resource.hourlyCost | substr 0 5) }}
{{- end }}
{{- end }}

Monthly Total: ${{ .Cost.breakdown.totalMonthlyCost }}
Hourly  Total: ${{ .Cost.breakdown.totalHourlyCost | substr 0 5 }}
{{- end }}

{{- end }}
{{- end }}
