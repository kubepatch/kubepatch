package assert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/yaml"
)

type CmdOptions struct {
	File    string
	Timeout time.Duration
}

type RunOptions struct {
	ConfigFlags *genericclioptions.ConfigFlags
	Streams     genericiooptions.IOStreams
	Cmd         CmdOptions
}

type Assertion struct {
	Assert    string      `yaml:"assert"`
	Namespace string      `yaml:"namespace,omitempty"`
	Field     string      `yaml:"field"`
	Equals    interface{} `yaml:"equals"`
}

func k() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
}

func Run(opts *RunOptions) error {
	cmdOptions := opts.Cmd

	ns := "default"
	if opts.ConfigFlags.Namespace != nil && *opts.ConfigFlags.Namespace != "" {
		ns = *opts.ConfigFlags.Namespace
	}

	// load assertions
	assertions, err := loadAssertions(cmdOptions.File)
	if err != nil {
		return err
	}

	// init clients
	cfg, err := k()
	if err != nil {
		return err
	}
	cfg.QPS = 50
	cfg.Burst = 100

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disc))
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return err
	}

	allPass := true
	for _, a := range assertions {
		ctx, cancel := context.WithTimeout(context.Background(), cmdOptions.Timeout)
		err := wait.PollUntilContextTimeout(ctx, 5*time.Second, cmdOptions.Timeout, true, func(ctx context.Context) (bool, error) {
			return evaluateAssertion(ctx, dyn, mapper, a, ns)
		})
		cancel()

		if err != nil {
			allPass = false
			fmt.Printf("❌ [%s] %s: failed\n", effectiveNamespace(a, ns), a.Assert)
		}
	}

	if !allPass {
		return fmt.Errorf("test-suite fail")
	}

	fmt.Println("✅ All assertions passed")
	return nil
}

func loadAssertions(path string) ([]Assertion, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Assertion
	err = yaml.Unmarshal(raw, &out)
	return out, err
}

func parseKindName(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid assert format: %q", s)
	}
	return strings.ToLower(parts[0]), parts[1], nil
}

func evaluateAssertion(ctx context.Context, dyn dynamic.Interface, mapper meta.RESTMapper, a Assertion, cliNS string) (bool, error) {
	ns := effectiveNamespace(a, cliNS)

	kind, namePattern, err := parseKindName(a.Assert)
	if err != nil {
		return false, err
	}

	gvr, err := resolveGVR(mapper, kind)
	if err != nil {
		return false, fmt.Errorf("unable to map kind: %w", err)
	}

	res := dyn.Resource(gvr).Namespace(ns)

	list, err := res.List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("list failed: %w", err)
	}

	found := 0
	for _, item := range list.Items {
		if matched, _ := filepath.Match(namePattern, item.GetName()); matched {
			found++
			val, ok, _ := unstructured.NestedFieldCopy(item.Object, strings.Split(a.Field, ".")...)
			if !ok {
				// Field not found — could be temporary (e.g., pod status not yet populated)
				return false, nil // fmt.Errorf("%s: field %s not found", item.GetName(), a.Field)
			}
			if !equals(val, a.Equals) {
				fmt.Fprintf(os.Stderr, "waiting: %s %s = %v (expected %v)\n", item.GetName(), a.Field, val, a.Equals)
				return false, nil // fmt.Errorf("%s: %s = %v (expected %v)", item.GetName(), a.Field, val, a.Equals)
			}
			fmt.Printf("✔ [%s] %s/%s: %s = %v\n", ns, gvr.Resource, item.GetName(), a.Field, val)
		}
	}
	if found == 0 {
		return false, fmt.Errorf("no resource matched pattern %q", namePattern)
	}
	return true, nil
}

func resolveGVR(mapper meta.RESTMapper, kind string) (schema.GroupVersionResource, error) {
	resources, err := mapper.ResourcesFor(schema.GroupVersionResource{Resource: kind})
	if err != nil || len(resources) == 0 {
		return schema.GroupVersionResource{}, err
	}
	return resources[0], nil
}

func effectiveNamespace(a Assertion, cliNS string) string {
	if a.Namespace != "" {
		return a.Namespace
	}
	if cliNS != "" {
		return cliNS
	}
	return "default"
}

func equals(a, b interface{}) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return bytes.Equal(aj, bj)
}
