// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package kind

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/cmd"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/random"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/docker"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/kubernetes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/log"
)

const (
	defaultClusterWaitTimeout = "30s"
)

var EmptyClusterInfo = ClusterInfo{}

// Client a kind client for managing kind clusters.
type Client interface {
	// CreateCluster creates a kind cluster and adds the context to the kubeconfig
	CreateCluster(request CreateKindClusterRequest) (ClusterInfo, error)
	// DeleteCluster deletes a kind cluster. This will stop the kind cluster and remove the context from
	// the kubeconfig
	DeleteCluster(request DeleteKindClusterRequest) error
	// LoadImageArchive loads a docker image into a kind cluster from an archive file
	LoadImageArchive(req LoadImageArchiveRequest) (LoadedImageResult, error)
	// LoadImage loads a docker image into a kind cluster
	LoadImage(req LoadImageRequest) (LoadedImageResult, error)
}

// localClient a kind client for managing local kind clusters.
type localClient struct {
	logger log.Logger
}

type CreateKindClusterRequest struct {
	Name           string
	KubeConfigPath string
	WaitTimeout    string
}

type ClusterInfo struct {
	KubeConfigBase64     string
	ContextName          string
	ClientCertificate    string
	ClientKey            string
	ClusterCACertificate string
	Endpoint             string
}

type DeleteKindClusterRequest struct {
	Name           string
	KubeConfigPath string
}

type LoadImageRequest struct {
	ClusterName string
	ImageName   string
	Tag         string
}

// GetImageRef gets the image ref for the request, i.e. name:tag.
func (l LoadImageRequest) GetImageRef() string {
	return fmt.Sprintf("%s:%s", l.ImageName, l.Tag)
}

// LoadedImageResult info about what cluster nodes an image was loaded on.
type LoadedImageResult struct {
	// Images the images that were loaded. Each image is loaded on each node
	Images []docker.ImageInfo
	// Nodes kind cluster control plane nodes where the images were loaded
	Nodes []string
}

// ID generates an id, by hashing the loaded image IDs, falls back to a random id if hashing fails.
func (l *LoadedImageResult) ID() string {
	h := fnv.New32a()

	for _, image := range l.Images {
		for _, tag := range image.Tags {
			if _, err := h.Write([]byte(tag.ID)); err != nil {
				// This should never happen, but just in case
				return random.ID()
			}
		}
	}

	return strconv.Itoa(int(h.Sum32()))
}

type LoadImageArchiveRequest struct {
	ClusterName  string
	ImageArchive string
}

func NewLocalClient(logger log.Logger) Client {
	return &localClient{logger: logger}
}

// CreateCluster creates a new kind cluster locally and returns the cluster info if successful.
func (c *localClient) CreateCluster(request CreateKindClusterRequest) (ClusterInfo, error) {
	if len(strings.TrimSpace(request.Name)) == 0 {
		return EmptyClusterInfo, errors.New("cannot create a cluster with an empty cluster 'name'")
	}

	logFields := map[string]any{"name": request.Name}

	var copts []cluster.CreateOption

	path, err := kubernetes.GetKubeConfigPath(request.KubeConfigPath)
	if err != nil {
		return EmptyClusterInfo, fmt.Errorf("failed to get the kubeconfig path while creating a cluster, due to: %w", err)
	}
	copts = append(copts, cluster.CreateWithKubeconfigPath(path))
	logFields["kubeconfig_path"] = path

	waitTimeout := defaultClusterWaitTimeout
	if len(strings.TrimSpace(request.WaitTimeout)) > 0 {
		waitTimeout = request.WaitTimeout
	}

	wait, err := time.ParseDuration(waitTimeout)
	if err != nil {
		return EmptyClusterInfo, fmt.Errorf("failed to parse 'wait_timeout': [%s], when creating cluster: [%s], due to: %w", waitTimeout, request.Name, err)
	}

	copts = append(copts, cluster.CreateWithWaitForReady(wait))

	c.logger.Info("Creating Local Kind Cluster", logFields)
	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))

	if err := provider.Create(request.Name, copts...); err != nil {
		return EmptyClusterInfo, err
	}

	c.logger.Info("Local Kind Cluster Created", logFields)

	return GetClusterInfo(request.Name)
}

// GetClusterInfo gets the cluster info for a local kind cluster.
func GetClusterInfo(name string) (ClusterInfo, error) {
	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))
	kubeConfig, err := provider.KubeConfig(name, false)
	if err != nil {
		return EmptyClusterInfo, fmt.Errorf("encountered error reading kube config: %s", err)
	}

	encodedKubeConfig := base64.StdEncoding.EncodeToString([]byte(kubeConfig))
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfig))
	if err != nil {
		return EmptyClusterInfo, fmt.Errorf("encountered error getting rest config: %s", err)
	}

	return ClusterInfo{
		KubeConfigBase64:     encodedKubeConfig,
		ContextName:          "kind-" + name,
		ClientCertificate:    string(config.CertData),
		ClientKey:            string(config.KeyData),
		ClusterCACertificate: string(config.CAData),
		Endpoint:             config.Host,
	}, nil
}

// DeleteCluster Deletes a local kind cluster.
func (c *localClient) DeleteCluster(request DeleteKindClusterRequest) error {
	if len(strings.TrimSpace(request.Name)) == 0 {
		return errors.New("cannot delete cluster without cluster 'name'")
	}

	kubeConfigPath, err := kubernetes.GetKubeConfigPath(request.KubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to destroy cluster, due to: %w", err)
	}

	c.logger.Info("Destroying Local Kind Cluster", map[string]any{
		"name":            request.Name,
		"kubeconfig_path": kubeConfigPath,
	})

	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))

	if err := provider.Delete(request.Name, kubeConfigPath); err != nil {
		return fmt.Errorf("failed to destroy cluster, due to: %w", err)
	}

	c.logger.Info("Local Kind Cluster Destroyed", map[string]any{
		"name":            request.Name,
		"kubeconfig_path": kubeConfigPath,
	})

	return nil
}

// LoadImageArchive Loads an image archive file into all nodes of a kind cluster as per the request.
func (c *localClient) LoadImageArchive(req LoadImageArchiveRequest) (LoadedImageResult, error) {
	return c.loadImageArchive(req.ImageArchive, req.ClusterName)
}

// LoadImage Loads an image into all nodes of a kind cluster as per the request.
func (c *localClient) LoadImage(req LoadImageRequest) (LoadedImageResult, error) {
	dir, err := os.MkdirTemp("", req.GetImageRef())
	if err != nil {
		return LoadedImageResult{}, fmt.Errorf("failed to create temporary directory when saving the image to an archive, due to: %w", err)
	}
	defer func() {
		os.RemoveAll(dir)
	}()

	imageName := req.ImageName
	tag := req.Tag
	imageTarName := imageName + "_" + tag + ".tar"
	imageTar := filepath.Join(dir, imageTarName)
	image := req.GetImageRef()

	// saved the docker image to a tar archive
	commandArgs := append([]string{"save", "-o", imageTar}, image)
	if err := exec.CommandContext(context.TODO(), "docker", commandArgs...).Run(); err != nil {
		return LoadedImageResult{}, fmt.Errorf("failed to export image: [%s] to archive: [%s], due to: %w", imageName, imageTar, err)
	}

	return c.loadImageArchive(imageTar, req.ClusterName)
}

// loadImageArchive loads the provided image archive onto all nodes of the provided cluster.
func (c *localClient) loadImageArchive(archive, clusterName string) (LoadedImageResult, error) {
	result := LoadedImageResult{Nodes: []string{}}

	infos, err := docker.GetImageInfos(archive)
	if err != nil {
		return result, fmt.Errorf("failed to get load image archive: [%s], due to: %w", archive, err)
	}
	result.Images = infos

	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))
	nodes, err := provider.ListInternalNodes(clusterName)
	if err != nil {
		return result, fmt.Errorf("failed to list nodes for cluster: [%s], due to: %w", clusterName, err)
	}

	if len(nodes) == 0 {
		return result, fmt.Errorf("no nodes found for cluster: [%s]", clusterName)
	}
	tarFile, err := os.Open(archive)
	if err != nil {
		return result, fmt.Errorf("failed to open image archive: [%s], due to: %w", archive, err)
	}
	defer tarFile.Close()

	for i, node := range nodes {
		if i != 0 {
			_, err := tarFile.Seek(0, io.SeekStart)
			if err != nil {
				return result, fmt.Errorf("failed to load image archive: [%s] to cluster: [%s], due to: %w", archive, clusterName, err)
			}
		}

		err = nodeutils.LoadImageArchive(node, tarFile)
		if err != nil {
			return result, fmt.Errorf("failed to load image archive: [%s] to cluster: [%s], due to: %w", archive, clusterName, err)
		}

		result.Nodes = append(result.Nodes, node.String())
	}

	return result, nil
}
