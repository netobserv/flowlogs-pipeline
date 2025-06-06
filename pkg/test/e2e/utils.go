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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vladimirvivien/gexe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

type namespaceContextKey string

type ManifestDeployDefinitions []ManifestDeployDefinition

type ManifestDeployDefinition struct {
	PreFunction  func(ctx context.Context, cfg *envconf.Config, namespace string) error
	YamlFile     string
	PostFunction func(ctx context.Context, cfg *envconf.Config, namespace string) error
}

func Main(m *testing.M, manifestDeployDefinitions ManifestDeployDefinitions, testEnv *env.Environment) {
	*testEnv = env.New()
	kindClusterName := "test"
	namespace := "default"
	org := "netobserv"
	version := "e2e"
	arch := "amd64"

	(*testEnv).Setup(
		e2eRecreateKindCluster(kindClusterName),
		e2eBuildAndLoadImageIntoKind(org, version, arch, kindClusterName),
		e2eRecreateNamespace(namespace),
		e2eDeployEnvironmentResources(manifestDeployDefinitions, namespace),
	)

	(*testEnv).Finish(
		e2eDeleteKindCluster(kindClusterName),
	)
	os.Exit((*testEnv).Run(m))
}

func e2eRecreateKindCluster(clusterName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("====> Recreating KIND cluster - %s\n", clusterName)
		gexe.RunProc(fmt.Sprintf(`kind delete cluster --name %s`, clusterName))
		newCtx, err := envfuncs.CreateCluster(kind.NewProvider(), clusterName)(ctx, cfg)
		fmt.Printf("====> Done.\n")
		return newCtx, err
	}
}

func e2eDeleteKindCluster(clusterName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		fmt.Printf("====> Deleting KIND cluster - %s\n", clusterName)
		newCtx, err := envfuncs.DestroyCluster(clusterName)(ctx, cfg)
		fmt.Printf("\n====> Done.\n")
		return newCtx, err
	}
}

func e2eBuildAndLoadImageIntoKind(org, version, arch, clusterName string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		e := gexe.New()
		fmt.Printf("====> building docker image - %s:%s\n", org, version)
		p := e.RunProc(fmt.Sprintf(`/bin/sh -c "cd $(git rev-parse --show-toplevel); MULTIARCH_TARGETS=%s IMAGE_ORG=%s VERSION=%s KIND_CLUSTER_NAME=%s make image-build kind-load-image"`,
			arch, org, version, clusterName))
		if p.Err() != nil || !p.IsSuccess() || p.ExitCode() != 0 {
			return nil, fmt.Errorf("failed to build or load docker image err=%w result=%v", p.Err(), p.Result())
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

		if name == "default" {
			fmt.Printf("====> Done. (Default namespace always exist)\n")
			cfg.WithNamespace(name)
			return context.WithValue(ctx, namespaceContextKey(name), namespace), err
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

		// create namespace
		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if err = client.Resources().Create(ctx, namespace); err != nil {
			return ctx, fmt.Errorf("create namespace func: %w", err)
		}

		fmt.Printf("====> Done.\n")
		cfg.WithNamespace(name)
		return context.WithValue(ctx, namespaceContextKey(name), namespace), err
	}
}

func e2eDeployEnvironmentResources(manifestDeployDefinitions ManifestDeployDefinitions, namespace string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		for _, manifestDeployDefinition := range manifestDeployDefinitions {
			if manifestDeployDefinition.PreFunction != nil {
				err := manifestDeployDefinition.PreFunction(ctx, cfg, namespace)
				if err != nil {
					return ctx, fmt.Errorf("e2eDeployEnvironmentResources:PreFunction: error: %w", err)
				}
			}
			err := deployManifest(ctx, cfg, namespace, manifestDeployDefinition.YamlFile)
			if err != nil {
				return ctx, fmt.Errorf("deployManifest failed: %w", err)
			}
			if manifestDeployDefinition.PostFunction != nil {
				err = manifestDeployDefinition.PostFunction(ctx, cfg, namespace)
				if err != nil {
					return ctx, fmt.Errorf("e2eDeployEnvironmentResources:PostFunction: error: %w", err)
				}
			}
		}

		return ctx, nil
	}
}

func deployManifest(_ context.Context, _ *envconf.Config, namespace string, yamlFileName string) error {
	fmt.Printf("====> Deploying k8s manifest - %s\n", yamlFileName)
	cmd := fmt.Sprintf("cat %s | kubectl apply -n %s -f -", yamlFileName, namespace)
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		fmt.Printf("deployManifest %s error :: %v\n", yamlFileName, err)
		fmt.Printf("  Stdeout: %v\n", string(out))
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			fmt.Printf("  Stderror: %v\n", string(exitErr.Stderr))
		}
	}

	fmt.Printf("====> Done.\n")
	return nil
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

func LogsFromPods(pods *corev1.PodList, coreV1Client *corev1client.CoreV1Client, namespace string) string {
	var logsBuilder strings.Builder
	for i := range pods.Items {
		pod := &pods.Items[i]
		req := coreV1Client.Pods(namespace).GetLogs(pod.GetName(), &corev1.PodLogOptions{Follow: false})
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
