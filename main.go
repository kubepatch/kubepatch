package main

import (
	"bytes"
	"encoding/json"
	"flag"
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
	When    string                   `yaml:"when"`
	Patches []map[string]interface{} `yaml:"patches"`
}

func main() {
	ctxArgs := flag.String("context", "", "Comma-separated context overrides (e.g. key=val,foo=bar)")
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		fmt.Println("Usage: katomik [--context key=val,...] <manifest-dir-or-file> <patches.yaml>")
		os.Exit(1)
	}

	manifestPath := args[0]
	patchFile := args[1]

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

	ctx := getEnvContext()
	if *ctxArgs != "" {
		overrides := strings.Split(*ctxArgs, ",")
		for _, kv := range overrides {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				ctx[parts[0]] = parts[1]
			}
		}
	}

	for i, doc := range manifests {
		meta := getMetadata(doc)
		if meta == nil {
			continue
		}
		for _, group := range patchGroups {
			if group.Target.Kind != meta.Kind || group.Target.Name != meta.Name {
				continue
			}
			if group.When != "" && !evaluateCondition(group.When, ctx) {
				continue
			}

			rawOps := []map[string]interface{}{}
			for _, patch := range group.Patches {
				if cond, ok := patch["when"].(string); ok {
					if !evaluateCondition(cond, ctx) {
						continue
					}
					delete(patch, "when")
				}
				rawOps = append(rawOps, patch)
			}

			if len(rawOps) == 0 {
				continue
			}

			jsonData, _ := json.Marshal(doc)
			patchJson, _ := json.Marshal(rawOps)

			if _, err := jsonpatch.DecodePatch(patchJson); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid patch for %s/%s: %v\n", meta.Kind, meta.Name, err)
				os.Exit(1)
			}

			patch, _ := jsonpatch.DecodePatch(patchJson)
			patchedJson, err := patch.Apply(jsonData)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to apply patch to %s/%s: %v\n", meta.Kind, meta.Name, err)
				os.Exit(1)
			}

			var updated map[string]interface{}
			_ = json.Unmarshal(patchedJson, &updated)
			manifests[i] = updated
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

func getEnvContext() map[string]string {
	ctx := map[string]string{}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			ctx[parts[0]] = parts[1]
		}
	}
	return ctx
}

func evaluateCondition(expr string, ctx map[string]string) bool {
	expr = strings.TrimSpace(expr)
	clauses := strings.Split(expr, "&&")
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if strings.Contains(clause, "!=") {
			parts := strings.SplitN(clause, "!=", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if ctx[key] == val {
				return false
			}
		} else if strings.Contains(clause, "==") {
			parts := strings.SplitN(clause, "==", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if ctx[key] != val {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
