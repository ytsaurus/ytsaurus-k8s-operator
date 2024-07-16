
{{/*
Expand the name of the cert-manager Issuer.
*/}}
{{- define "ytop-chart.issuerRefName" -}}
{{- default (printf "%s-selfsigned-issuer" (include "ytop-chart.fullname" .)) .Values.issuerRef.name }}
{{- end }}
