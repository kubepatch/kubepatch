package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"sigs.k8s.io/yaml"
)

type PatchTarget struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

type PatchGroup struct {
	Target  PatchTarget              `yaml:"target"`
	Patches []map[string]interface{} `yaml:"patches"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: katomik <manifest-dir-or-file> <patches.yaml>")
		os.Exit(1)
	}

	manifestPath := os.Args[1]
	patchFile := os.Args[2]

	manifests, err := loadManifests(manifestPath)
	if err != nil {
		panic(err)
	}

	patchData, err := os.ReadFile(patchFile)
	if err != nil {
		panic(err)
	}

	var patchGroups []PatchGroup
	if err := yaml.Unmarshal(patchData, &patchGroups); err != nil {
		panic(err)
	}

	for i, doc := range manifests {
		meta := getMetadata(doc)
		if meta == nil {
			continue
		}
		for _, group := range patchGroups {
			if group.Target.Kind == meta.Kind && group.Target.Name == meta.Name {
				jsonData, _ := json.Marshal(doc)
				patchJson, _ := json.Marshal(group.Patches)
				patch, err := jsonpatch.DecodePatch(patchJson)
				if err != nil {
					panic(err)
				}
				patchedJson, err := patch.Apply(jsonData)
				if err != nil {
					panic(err)
				}
				var updated map[string]interface{}
				_ = json.Unmarshal(patchedJson, &updated)
				manifests[i] = updated
			}
		}
	}

	for _, doc := range manifests {
		out, _ := yaml.Marshal(doc)
		fmt.Printf("---\n%s", string(out))
	}
}

type Metadata struct {
	Kind string
	Name string
}

func getMetadata(obj map[string]interface{}) *Metadata {
	kind, _ := obj["kind"].(string)
	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	name, _ := meta["name"].(string)
	return &Metadata{Kind: kind, Name: name}
}

func loadManifests(path string) ([]map[string]interface{}, error) {
	var manifests []map[string]interface{}
	files := []string{}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		err := filepath.Walk(path, func(p string, info fs.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".yaml") {
				return nil
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = append(files, path)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		yamlDocs := bytes.Split(data, []byte("---"))
		for _, doc := range yamlDocs {
			doc = bytes.TrimSpace(doc)
			if len(doc) == 0 {
				continue
			}
			var obj map[string]interface{}
			if err := yaml.Unmarshal(doc, &obj); err != nil {
				return nil, err
			}
			manifests = append(manifests, obj)
		}
	}

	return manifests, nil
}
