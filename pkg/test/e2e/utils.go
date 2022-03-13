/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vladimirvivien/gexe"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

type namespaceContextKey string

func Main(m *testing.M, TestEnv *env.Environment) {
	*TestEnv = env.New()
	kindClusterName := "test"
	namespace := "test"
	yamlFiles := []string{"k8s-objects.yaml"}
	dockerImage := "quay.io/netobserv/flowlogs-pipeline"
	dockerTag := "e2e"

	(*TestEnv).Setup(
		e2eCreateKindCluster(kindClusterName),
		e2eBuildAndLoadImageIntoKind(dockerImage, dockerTag, kindClusterName),
		e2eRecreateNamespace(namespace),
		e2eDeployEnvironmentResources(yamlFiles, namespace),
	)

	(*TestEnv).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit((*TestEnv).Run(m))
}

func e2eCreateKindCluster(clusterName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("====> Creating KIND cluster - %s\n", clusterName)
		newCtx, err := envfuncs.CreateKindCluster(clusterName)(ctx, cfg)
		fmt.Printf("====> Done.\n")
		return newCtx, err
	}
}

func e2eBuildAndLoadImageIntoKind(dockerImg, dockerTag, clusterName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		e := gexe.New()
		fmt.Printf("====> building docker image - %s:%s\n", dockerImg, dockerTag)
		p := e.RunProc(fmt.Sprintf(`/bin/sh -c "cd $(git rev-parse --show-toplevel); OCI_RUNTIME=docker DOCKER_IMG=%s DOCKER_TAG=%s make build-image"`,
			dockerImg, dockerTag))
		if p.Err() != nil || !p.IsSuccess() || p.ExitCode() != 0 {
			return nil, fmt.Errorf("failed to building docker image err=%v result=%v", p.Err(), p.Result())
		}
		fmt.Printf("====> Done.\n")
		fmt.Printf("====> Load image into kind \n")
		p = e.RunProc(fmt.Sprintf("kind load --name %s docker-image %s:%s", clusterName, dockerImg, dockerTag))
		if p.Err() != nil || !p.IsSuccess() || p.ExitCode() != 0 {
			return nil, fmt.Errorf("failed to load image into kind err=%v result=%v", p.Err(), p.Result())
		}
		fmt.Printf("====> Done.\n")
		return ctx, nil
	}

}

func e2eRecreateNamespace(name string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("====> Recreating namespace - %s\n", name)
		var namespace *corev1.Namespace

		// attempt to retrieve from context
		nsVal := ctx.Value(namespaceContextKey(name))
		if nsVal != nil {
			if ns, ok := nsVal.(*corev1.Namespace); ok {
				namespace = ns
			}
		}

		client, err := cfg.NewClient()
		if err != nil {
			return ctx, fmt.Errorf("delete namespace func: %w", err)
		}

		// if not in context, get from server
		if namespace == nil {
			var ns corev1.Namespace
			if err = client.Resources().Get(ctx, name, name, &ns); err == nil {
				namespace = &ns
			}
		}

		// remove if exists
		if namespace != nil {
			// remove namespace api object
			if err = client.Resources().Delete(ctx, namespace); err != nil {
				return ctx, fmt.Errorf("delete namespace func: %w", err)
			}

			err = wait.For(conditions.New(client.Resources()).ResourceDeleted(namespace), wait.WithTimeout(time.Minute*1))
			if err != nil {
				return ctx, fmt.Errorf("waiting for delete namespace func: %w", err)
			}
		}

		//create namespace
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if err = client.Resources().Create(ctx, namespace); err != nil {
			return ctx, fmt.Errorf("create namespace func: %w", err)
		}

		fmt.Printf("====> Done.\n")
		cfg.WithNamespace(name)
		return context.WithValue(ctx, namespaceContextKey(name), namespace), err
	}
}

func e2eDeployEnvironmentResources(yamlFiles []string, namespace string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("====> Deploying k8s resources from - %v\n", yamlFiles)
		yaml := ""
		for _, file := range yamlFiles {
			fileYaml, err := ioutil.ReadFile(file)
			if err != nil {
				return ctx, fmt.Errorf("e2eDeployEnvironmentResources - ReadFile: %w", err)
			}

			yaml += "\n---\n" + string(fileYaml)
		}

		resources := parseK8sYaml(string(yaml))
		client, err := cfg.NewClient()
		if err != nil {
			return ctx, fmt.Errorf("e2eDeployEnvironmentResources - NewClient: %w", err)
		}

		for _, resource := range resources {
			k8sResource := resource.(k8s.Object)
			k8sResource.SetNamespace(namespace)

			// Recreate resource
			// Note: Skip if resource do not exist, we create
			_ = client.Resources().Delete(ctx, k8sResource)
			err = client.Resources().Create(ctx, k8sResource)
			if err != nil {
				return ctx, fmt.Errorf("create resource : %w", err)
			}
		}
		fmt.Printf("====> Done.\n")
		return ctx, nil
	}
}

func parseK8sYaml(yaml string) []runtime.Object {
	acceptedK8sTypes := regexp.MustCompile(`(ConfigMap|Service|Deployment|ServiceAccount|ClusterRole|ClusterRoleBinding|Secret)`)
	sepYamlFiles := strings.Split(yaml, "---")
	resources := make([]runtime.Object, 0, len(sepYamlFiles))
	for _, f := range sepYamlFiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode
		resource, groupVersionKind, err := decode([]byte(f), nil, nil)

		if err != nil {
			continue
		}

		if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
		} else {
			resources = append(resources, resource)
		}
	}
	return resources
}

func GetConfigSet(confFileName string) (*corev1client.CoreV1Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", confFileName)
	if err != nil {
		return nil, err
	}

	clientSet, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientSet, err
}

func LogsFromPods(pods corev1.PodList, clientSet *corev1client.CoreV1Client, namespace string) string {
	logs := ""
	for _, pod := range pods.Items {
		req := clientSet.Pods(namespace).GetLogs(pod.GetName(), &v1.PodLogOptions{Follow: false})
		podLogs, err := req.Stream(context.TODO())
		if err != nil {
			// skipping
			continue
		}
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			// skipping
			continue
		}
		str := buf.String()
		logs += fmt.Sprintf("Logs from pod %s\n----\n%s\n", pod.GetName(), str)
	}

	return logs
}
