package resolver

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/runtime-spec/specs-go"
	faasd "github.com/openfaas/faasd/pkg"
	"github.com/openfaas/faasd/pkg/cninetwork"
)

const annotationLabelPrefix = "com.openfaas.annotations."

type Function struct {
	Name        string
	Namespace   string
	Image       string
	PID         uint32
	Replicas    int
	IP          string
	Labels      map[string]string
	Annotations map[string]string
	Secrets     []string
	EnvVars     map[string]string
	EnvProcess  string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

type Resolver struct {
	containerd   *containerd.Client
	FunctionURLs *sync.Map
}

func New(
	containerd *containerd.Client,
) *Resolver {
	r := &Resolver{
		containerd:   containerd,
		FunctionURLs: &sync.Map{},
	}

	go func() {
		t := time.NewTicker(2 * time.Second)
		for range t.C {
			functions, err := r.listFunctions()
			if err != nil {
				log.Println("getting functions: %w", err)
			}

			for _, function := range functions {
				function.ExpiresAt = time.Now().Add(4 * time.Second)
				r.FunctionURLs.Store(function.Name, function)
			}
			r.FunctionURLs.Range(func(key, value interface{}) bool {
				function := value.(*Function)
				if function.ExpiresAt.Before(time.Now()) {
					r.FunctionURLs.Delete(key)
				}
				return true
			})
		}
	}()

	return r
}

func (r *Resolver) Resolve(name string) (string, bool) {
	v, ok := r.FunctionURLs.Load(name)
	if !ok {
		return "", false
	}

	function := v.(*Function)

	return "http://" + function.IP + ":8080", true
}

// ListFunctions returns a map of all functions with running tasks on namespace
func (r *Resolver) listFunctions() (map[string]*Function, error) {
	ctx := namespaces.WithNamespace(context.Background(), faasd.DefaultFunctionNamespace)
	functions := make(map[string]*Function)

	containers, err := r.containerd.Containers(ctx)
	if err != nil {
		return functions, err
	}

	for _, c := range containers {
		name := c.ID()
		f, err := r.getFunction(name)
		if err != nil {
			log.Printf("error getting function %s: ", name)
			return functions, err
		}
		functions[name] = &f
	}

	return functions, nil
}

// GetFunction returns a function that matches name
func (r *Resolver) getFunction(name string) (Function, error) {
	ctx := namespaces.WithNamespace(context.Background(), faasd.DefaultFunctionNamespace)
	fn := Function{}

	c, err := r.containerd.LoadContainer(ctx, name)
	if err != nil {
		return Function{}, fmt.Errorf("unable to find function: %s, error %s", name, err)
	}

	image, err := c.Image(ctx)
	if err != nil {
		return fn, err
	}

	containerName := c.ID()
	allLabels, labelErr := c.Labels(ctx)

	if labelErr != nil {
		log.Printf("cannot list container %s labels: %s", containerName, labelErr.Error())
	}

	labels, annotations := buildLabelsAndAnnotations(allLabels)

	spec, err := c.Spec(ctx)
	if err != nil {
		return Function{}, fmt.Errorf("unable to load function spec for reading secrets: %s, error %s", name, err)
	}

	info, err := c.Info(ctx)
	if err != nil {
		return Function{}, fmt.Errorf("can't load info for: %s, error %s", name, err)
	}

	envVars, envProcess := readEnvFromProcessEnv(spec.Process.Env)
	secrets := readSecretsFromMounts(spec.Mounts)

	fn.Name = containerName
	fn.Namespace = faasd.DefaultFunctionNamespace
	fn.Image = image.Name()
	fn.Labels = labels
	fn.Annotations = annotations
	fn.Secrets = secrets
	fn.EnvVars = envVars
	fn.EnvProcess = envProcess
	fn.CreatedAt = info.CreatedAt

	replicas := 0
	task, err := c.Task(ctx, nil)
	if err == nil {
		// Task for container exists
		svc, err := task.Status(ctx)
		if err != nil {
			return Function{}, fmt.Errorf("unable to get task status for container: %s %s", name, err)
		}

		if svc.Status == "running" {
			replicas = 1
			fn.PID = task.Pid()

			// Get container IP address
			ip, err := cninetwork.GetIPAddress(name, task.Pid())
			if err != nil {
				return Function{}, err
			}
			fn.IP = ip
		}
	} else {
		replicas = 0
	}

	fn.Replicas = replicas
	return fn, nil
}

func readEnvFromProcessEnv(env []string) (map[string]string, string) {
	foundEnv := make(map[string]string)
	fprocess := ""
	for _, e := range env {
		kv := strings.Split(e, "=")
		if len(kv) == 1 {
			continue
		}

		if kv[0] == "PATH" {
			continue
		}

		if kv[0] == "fprocess" {
			fprocess = kv[1]
			continue
		}

		foundEnv[kv[0]] = kv[1]
	}

	return foundEnv, fprocess
}

func readSecretsFromMounts(mounts []specs.Mount) []string {
	secrets := []string{}
	for _, mnt := range mounts {
		x := strings.Split(mnt.Destination, "/var/openfaas/secrets/")
		if len(x) > 1 {
			secrets = append(secrets, x[1])
		}

	}
	return secrets
}

// buildLabelsAndAnnotations returns a separated list with labels first,
// followed by annotations by checking each key of ctrLabels for a prefix.
func buildLabelsAndAnnotations(ctrLabels map[string]string) (map[string]string, map[string]string) {
	labels := make(map[string]string)
	annotations := make(map[string]string)

	for k, v := range ctrLabels {
		if strings.HasPrefix(k, annotationLabelPrefix) {
			annotations[strings.TrimPrefix(k, annotationLabelPrefix)] = v
		} else {
			labels[k] = v
		}
	}

	return labels, annotations
}
