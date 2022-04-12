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

	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	log "github.com/sirupsen/logrus"
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

type YamlInfo struct {
	YamlFile    string
	Namespace   string
	PreCommands []string
}

func Main(m *testing.M, yamlInfos []YamlInfo, testEnv *env.Environment) {
	*testEnv = env.New()
	kindClusterName := "test"
	namespace := "test"
	dockerImage := "quay.io/netobserv/flowlogs-pipeline"
	dockerTag := "e2e"

	(*testEnv).Setup(
		e2eCreateKindCluster(kindClusterName),
		E2eBuildAndLoadImageIntoKind(dockerImage, dockerTag, kindClusterName),
		e2eRecreateNamespace(namespace),
		e2eDeployEnvironmentResources(yamlInfos),
	)

	(*testEnv).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit((*testEnv).Run(m))
}

func e2eCreateKindCluster(clusterName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("====> Creating KIND cluster - %s\n", clusterName)
		newCtx, err := envfuncs.CreateKindCluster(clusterName)(ctx, cfg)
		fmt.Printf("====> Done.\n")
		return newCtx, err
	}
}

// NOTE: the e2e framework uses `kind load` command that forces usage of docker (and not podman)
// ref: https://github.com/kubernetes-sigs/kind/issues/2027
func E2eBuildAndLoadImageIntoKind(dockerImg, dockerTag, clusterName string) env.Func {
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

func e2eDeployEnvironmentResources(yamlInfos []YamlInfo) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		client, err := cfg.NewClient()
		if err != nil {
			return ctx, fmt.Errorf("e2eDeployEnvironmentResources - NewClient: %w", err)
		}
		for _, yInfo := range yamlInfos {
			for _, cmd := range yInfo.PreCommands {
				if cmd != "" {
					fmt.Printf("====> Performing Command: %s\n", cmd)
					_, err := test.RunCommand(cmd)
					if err != nil {
						fmt.Printf("error when running command: %v", err)
						return ctx, err
					}
				}
			}
			file := yInfo.YamlFile
			namespace := yInfo.Namespace
			fmt.Printf("====> Deploying k8s resources from - %s in namespace %s\n", file, namespace)
			fileYaml, err := ioutil.ReadFile(file)
			if err != nil {
				return ctx, fmt.Errorf("e2eDeployEnvironmentResources - ReadFile: %w", err)
			}

			resources := parseK8sYaml(string(fileYaml))

			for _, resource := range resources {
				k8sResource := resource.(k8s.Object)
				k8sResource.SetNamespace(namespace)

				// Recreate resource
				// Note: Skip if resource do not exist, we create
				_ = client.Resources().Delete(ctx, k8sResource)
				err = client.Resources().Create(ctx, k8sResource)
				//err = client.Resources().Update(ctx, k8sResource)
				if err != nil {
					return ctx, fmt.Errorf("create resource : %w", err)
				}
			}
			fmt.Printf("====> Done.\n")
		}
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

func GetCoreV1Client(confFileName string) (*corev1client.CoreV1Client, error) {
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

func LogsFromPods(pods corev1.PodList, coreV1Client *corev1client.CoreV1Client, namespace string) string {
	var logsBuilder strings.Builder
	for _, pod := range pods.Items {
		req := coreV1Client.Pods(namespace).GetLogs(pod.GetName(), &v1.PodLogOptions{Follow: false})
		podLogs, err := req.Stream(context.TODO())
		if err != nil {
			log.Errorf("req.Stream from pod %s error %v", pod.GetName(), err)
			continue
		}
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			log.Errorf("io.Copy from pod %s error %v", pod.GetName(), err)
			continue
		}
		logsBuilder.WriteString(fmt.Sprintf("Logs from pod %s\n----\n%s\n", pod.GetName(), buf.String()))
	}

	return logsBuilder.String()
}
