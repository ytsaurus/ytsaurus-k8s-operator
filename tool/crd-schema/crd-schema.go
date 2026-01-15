package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"sigs.k8s.io/yaml"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal(data, &crd); err != nil {
			panic(err)
		}

		for _, version := range crd.Spec.Versions {
			schema, err := json.MarshalIndent(version.Schema.OpenAPIV3Schema, "", "  ")
			if err != nil {
				panic(err)
			}
			schemaFile := fmt.Sprintf("%s_%s_%s.json", crd.Spec.Group, crd.Spec.Names.Singular, version.Name)
			outPath := path.Join(outDir, schemaFile)
			if err := os.WriteFile(outPath, []byte(schema), 0o644); err != nil {
				panic(err)
			}
		}
	}
}
