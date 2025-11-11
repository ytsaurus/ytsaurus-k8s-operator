package main

import (
	"os"
	"path"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		panic("not enough arguments")
	}
	outDir := os.Args[1]
	for _, inPath := range os.Args[2:] {
		data, err := os.ReadFile(inPath)
		if err != nil {
			panic(err)
		}
		out := strings.Builder{}
		out.WriteString("{{- if .Values.crds.enabled }}\n")
		annotationsFound := 0
		for line := range strings.SplitAfterSeq(string(data), "\n") {
			out.WriteString(line)
			if line == "  annotations:\n" {
				out.WriteString("    {{- if .Values.crds.keep }}\n    helm.sh/resource-policy: keep\n    {{- end }}\n")
				annotationsFound += 1
			}
		}
		if annotationsFound != 1 {
			panic("annotations")
		}
		out.WriteString("{{- end }}\n")
		outPath := path.Join(outDir, path.Base(inPath))
		if err := os.WriteFile(outPath, []byte(out.String()), 0o644); err != nil {
			panic(err)
		}
	}
}
