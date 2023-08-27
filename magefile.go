//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/magefile/mage/mg"
	"github.com/mt-sre/devkube/dev"
	"github.com/mt-sre/devkube/magedeps"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	aoapisv1alpha1 "github.com/openshift/hypershift-logging-operator/api/v1alpha1"
)

const (
	module          = "github.com/openshift/hypershift-logging-operator"
	defaultImageOrg = "quay.io/app-sre"
)

// Directories
var (
	// Working directory of the project.
	workDir string
	// Dependency directory.
	depsDir  magedeps.DependencyDirectory
	cacheDir string

	logger           logr.Logger
	containerRuntime string
)

func init() {
	var err error
	// Directories
	workDir, err = os.Getwd()
	if err != nil {
		panic(fmt.Errorf("getting work dir: %w", err))
	}
	cacheDir = path.Join(workDir + "/" + ".cache")
	depsDir = magedeps.DependencyDirectory(path.Join(workDir, ".deps"))
	os.Setenv("PATH", depsDir.Bin()+":"+os.Getenv("PATH"))

	logger = stdr.New(nil)

}

// dependency for all targets requiring a container runtime
func setupContainerRuntime() {
	containerRuntime = os.Getenv("CONTAINER_RUNTIME")
	if len(containerRuntime) == 0 || containerRuntime == "auto" {
		cr, err := dev.DetectContainerRuntime()
		if err != nil {
			panic(err)
		}
		containerRuntime = string(cr)
		logger.Info("detected container-runtime", "container-runtime", containerRuntime)
	}
}

func labelNodesWithInfraRole(ctx context.Context, cluster *dev.Cluster) error {
	nodeList := &corev1.NodeList{}
	if err := cluster.CtrlClient.List(ctx, nodeList); err != nil {
		return fmt.Errorf("listing nodes: %w", err)
	}
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}
		node.Labels["node-role.kubernetes.io/infra"] = "True"
		if err := cluster.CtrlClient.Update(ctx, node); err != nil {
			return fmt.Errorf("adding infra role to all nodes: %w", err)
		}
	}
	return nil
}

// Dependencies
// ------------

// Dependency Versions
const (
	controllerGenVersion = "0.6.2"
	kindVersion          = "0.11.1"
)

type Dependency mg.Namespace

func (d Dependency) All() {
	mg.Deps(
		Dependency.Kind,
		Dependency.ControllerGen,
	)
}

// Ensure Kind dependency - Kubernetes in Docker (or Podman)
func (d Dependency) Kind() error {
	return depsDir.GoInstall("kind",
		"sigs.k8s.io/kind", kindVersion)
}

// Ensure controller-gen - kubebuilder code and manifest generator.
func (d Dependency) ControllerGen() error {
	return depsDir.GoInstall("controller-gen",
		"sigs.k8s.io/controller-tools/cmd/controller-gen", controllerGenVersion)
}

// Development
// --------
type Dev mg.Namespace

var (
	devEnvironment *dev.Environment
)

func (d Dev) Setup(ctx context.Context) error {
	if err := d.init(); err != nil {
		return err
	}

	if err := devEnvironment.Init(ctx); err != nil {
		return fmt.Errorf("initializing dev environment: %w", err)
	}

	return nil
}

func (d Dev) Teardown(ctx context.Context) error {
	if err := d.init(); err != nil {
		return err
	}

	if err := devEnvironment.Destroy(ctx); err != nil {
		return fmt.Errorf("tearing down dev environment: %w", err)
	}
	return nil
}

func (d Dev) LoadImage(image string) error {

	imageTar := path.Join(cacheDir, "image", image+".tar")
	if err := devEnvironment.LoadImageFromTar(imageTar); err != nil {
		return fmt.Errorf("load image from tar: %w", err)
	}
	return nil
}

// Deploy the Addon Operator, and additionally the Mock API Server and Addon Operator webhooks if the respective
// environment variables are set.
// All components are deployed via static manifests.
func (d Dev) Deploy(ctx context.Context) error {
	mg.Deps(
		Dev.Setup, // setup is a pre-requesite and needs to run before we can load images.
	)

	if err := labelNodesWithInfraRole(ctx, devEnvironment.Cluster); err != nil {
		return err
	}

	mg.Deps(
		mg.F(Dev.LoadImage, "api-mock"),
		mg.F(Dev.LoadImage, "addon-operator-manager"),
	)

	if err := d.deploy(ctx, devEnvironment.Cluster); err != nil {
		return fmt.Errorf("deploying: %w", err)
	}
	return nil
}

// Deploy all addon operator components to a cluster.
func (d Dev) deploy(
	ctx context.Context, cluster *dev.Cluster,
) error {

	if err := d.deployAddonOperatorManager(ctx, cluster); err != nil {
		return err
	}

	return nil
}

// deploy the Addon Operator Manager from local files.
func (d Dev) deployAddonOperatorManager(ctx context.Context, cluster *dev.Cluster) error {
	deployment := &appsv1.Deployment{}
	err := loadAndConvertIntoObject(cluster.Scheme, "config/deploy/deployment.yaml.tpl", deployment)
	if err != nil {
		return fmt.Errorf("loading addon-operator-manager deployment.yaml.tpl: %w", err)
	}

	// Replace image
	//patchDeployment(deployment, "addon-operator-manager", "manager")

	ctx = logr.NewContext(ctx, logger)

	// Deploy
	if err := cluster.CreateAndWaitFromFiles(ctx, []string{
		// TODO: replace with CreateAndWaitFromFolders when deployment.yaml is gone.
		"config/deploy/00-namespace.yaml",
		"config/deploy/01-metrics-server-tls-secret.yaml",
		"config/deploy/addons.managed.openshift.io_addoninstances.yaml",
		"config/deploy/addons.managed.openshift.io_addonoperators.yaml",
		"config/deploy/addons.managed.openshift.io_addons.yaml",
		"config/deploy/metrics.service.yaml",
		"config/deploy/rbac.yaml",
		"config/deploy/trusted_ca_bundle_configmap.yaml",
	}); err != nil {
		return fmt.Errorf("deploy addon-operator-manager dependencies: %w", err)
	}

	if err := cluster.CreateAndWaitForReadiness(ctx, deployment); err != nil {
		return fmt.Errorf("deploy addon-operator-manager: %w", err)
	}
	return nil
}

func loadAndConvertIntoObject(scheme *k8sruntime.Scheme, filePath string, out interface{}) error {
	objs, err := dev.LoadKubernetesObjectsFromFile(filePath)
	if err != nil {
		return fmt.Errorf("loading object from file: %w", err)
	}
	if err := scheme.Convert(&objs[0], out, nil); err != nil {
		return fmt.Errorf("converting: %w", err)
	}
	return nil
}

func (d Dev) init() error {
	mg.SerialDeps(
		setupContainerRuntime,
		Dependency.Kind,
	)

	devEnvironment = dev.NewEnvironment(
		"hypershift-logging-operator-dev",
		path.Join(cacheDir, "dev-env"),
		dev.WithClusterOptions([]dev.ClusterOption{
			dev.WithWaitOptions([]dev.WaitOption{
				dev.WithTimeout(10 * time.Minute),
			}),
			dev.WithSchemeBuilder(runtime.SchemeBuilder{operatorsv1alpha1.AddToScheme, aoapisv1alpha1.AddToScheme}),
		}),
		dev.WithContainerRuntime(containerRuntime),
		dev.WithKindClusterConfig(kindv1alpha4.Cluster{
			Nodes: []kindv1alpha4.Node{
				{
					Role: kindv1alpha4.ControlPlaneRole,
				},
				/*
					## Uncomment this if you need worker nodes to the cluster
						{
							Role: kindv1alpha4.WorkerRole,
						},
						{
							Role: kindv1alpha4.WorkerRole,
						},
				*/
			},
		}),
	)
	return nil
}
