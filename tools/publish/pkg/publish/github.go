// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v60/github"
	"go.uber.org/zap"
)

type GithubReleaseCreateReq struct {
	// Local artifacts
	DistDir         string
	BinaryName      string
	ProviderName    string
	GPGKeyID        string
	GPGIdentityName string
	Version         string
	ManifestPath    string
	// Github release
	GithubRelease       *github.RepositoryRelease
	GithubToken         string
	GithubRepoNameOwner string // owner/repo
}

func (r *GithubReleaseCreateReq) OwnerRepo() (string, string) {
	parts := strings.Split(r.GithubRepoNameOwner, "/")
	return parts[0], parts[1]
}

type githubClient struct {
	Log *zap.SugaredLogger
	c   *github.Client
}

func withGithubLog(log *zap.SugaredLogger) githubClientOpt {
	return func(c *githubClient) {
		c.Log = log
		c.c = github.NewClient(nil)
	}
}

type githubClientOpt func(*githubClient)

func newGithubClient(opts ...githubClientOpt) *githubClient {
	c := &githubClient{
		Log: zap.NewExample().Sugar(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (g *githubClient) createRelease(
	ctx context.Context,
	req *GithubReleaseCreateReq,
) (*github.RepositoryRelease, *github.Response, error) {
	owner, repo := req.OwnerRepo()

	g.Log.Infow("creating github release",
		"owner", owner,
		"repo", repo,
		"req", fmt.Sprintf("%+v", req.GithubRelease),
	)

	return g.c.WithAuthToken(req.GithubToken).Repositories.CreateRelease(
		ctx, owner, repo, req.GithubRelease,
	)
}

func (g *githubClient) uploadAsset(
	ctx context.Context,
	req *GithubReleaseCreateReq,
	rel *github.RepositoryRelease,
	path string,
) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	owner, repo := req.OwnerRepo()
	opts := &github.UploadOptions{
		Name: filepath.Base(path),
	}

	g.Log.Infow("uploading github release asset",
		"owner", owner,
		"repo", repo,
		"id", rel.GetID(),
		"file", path,
	)

	_, _, err = g.c.WithAuthToken(req.GithubToken).Repositories.UploadReleaseAsset(
		ctx, owner, repo, rel.GetID(), opts, f,
	)

	return err
}
