package main

import (
	"os"
	"path"
	"strings"
)

func main() {
	outDir := os.Args[1]
	for _, inPath := range os.Args[2:] {
		data, err := os.ReadFile(inPath)
		if err != nil {
			panic(err)
		}
		out := strings.Builder{}
		out.WriteString("{{- if .Values.crds.enabled }}\n")
		annotationsFound := false
		for line := range strings.SplitAfterSeq(string(data), "\n") {
			out.WriteString(line)
			if line == "  annotations:\n" {
				out.WriteString("    {{- if .Values.crds.keep }}\n    helm.sh/resource-policy: keep\n    {{- end }}\n")
				annotationsFound = true
			}
		}
		if !annotationsFound {
			panic("no annotations found")
		}
		out.WriteString("{{- end }}\n")
		outPath := path.Join(outDir, path.Base(inPath))
		if err := os.WriteFile(outPath, []byte(out.String()), 0o644); err != nil {
			panic(err)
		}
	}
}
