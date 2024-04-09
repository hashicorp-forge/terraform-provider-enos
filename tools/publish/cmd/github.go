// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"os"

	"github.com/google/go-github/v60/github"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"

	"github.com/hashicorp-forge/terraform-provider-enos/tools/publish/pkg/publish"
)

var (
	githubTargetCommitish      string
	githubMakeLatest           string
	githubGenerateReleaseNotes bool
)

var githubReleaseCreateReq = &publish.GithubReleaseCreateReq{
	GithubRelease: &github.RepositoryRelease{
		TargetCommitish:      &githubTargetCommitish,
		MakeLatest:           &githubMakeLatest,
		GenerateReleaseNotes: &githubGenerateReleaseNotes,
	},
}

func newGithubCmd() *cobra.Command {
	ghCmd := &cobra.Command{
		Use:   "github [COMMANDS]",
		Short: "provider actions for github releases",
	}

	ghCmd.AddCommand(newGithubReleaseCreateCmd())

	return ghCmd
}

func newGithubReleaseCreateCmd() *cobra.Command {
	ghReleaseCreateCmd := &cobra.Command{
		Use:   "create [ARGS]",
		Short: "Create a Github release of the provider artifacts",
		RunE:  runGHReleaseCreateCmd,
	}

	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.DistDir, "dist", "", "the output directory of go build artifacts")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.ProviderName, "provider-name", "enos", "the name of the provider")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.BinaryName, "binary-name", "terraform-provider-enos", "the name of the provider binary")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.ManifestPath, "manifest-path", "terraform-registry-manifest.json", "the desired provider binary name during upload")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.GPGKeyID, "gpg-key-id", "", "the GPG Signing Key")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.GPGIdentityName, "gpg-identity-name", "team-vault-quality@hashicorp.com", "the GPG identity name, should be an email address")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.Version, "version", "", "the version of the provider to release")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.GithubRepoNameOwner, "repo", "hashicorp-forge/terraform-provider-enos", "the repository to publish to")
	ghReleaseCreateCmd.PersistentFlags().StringVar(&githubReleaseCreateReq.GithubToken, "token", os.Getenv("GITHUB_TOKEN"), "The Github Token with write access")
	ghReleaseCreateCmd.PersistentFlags().StringVar(githubReleaseCreateReq.GithubRelease.TargetCommitish, "commit", "", "the Github commit to release")
	ghReleaseCreateCmd.PersistentFlags().StringVar(githubReleaseCreateReq.GithubRelease.MakeLatest, "latest", "true", "make the release the latest")
	ghReleaseCreateCmd.PersistentFlags().BoolVar(githubReleaseCreateReq.GithubRelease.GenerateReleaseNotes, "release-notes", true, "Generate release notes")

	_ = ghReleaseCreateCmd.MarkFlagRequired("dist")
	_ = ghReleaseCreateCmd.MarkFlagRequired("provider-name")
	_ = ghReleaseCreateCmd.MarkFlagRequired("manifest-path")
	_ = ghReleaseCreateCmd.MarkFlagRequired("gpg-key-id")
	_ = ghReleaseCreateCmd.MarkFlagRequired("gpg-identity-name")
	_ = ghReleaseCreateCmd.MarkFlagRequired("version")
	_ = ghReleaseCreateCmd.MarkFlagRequired("repo")
	_ = ghReleaseCreateCmd.MarkFlagRequired("commit")
	_ = ghReleaseCreateCmd.MarkFlagRequired("latest")

	return ghReleaseCreateCmd
}

func runGHReleaseCreateCmd(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.requestTimeout)
	defer cancel()

	// Populate our release with some filler
	githubReleaseCreateReq.GithubRelease.Name = &githubReleaseCreateReq.Version
	tagName := "v" + githubReleaseCreateReq.Version // v0.5.0
	githubReleaseCreateReq.GithubRelease.TagName = &tagName
	body := githubReleaseCreateReq.BinaryName + "_" + tagName
	githubReleaseCreateReq.GithubRelease.Body = &body

	stagedRelease := publish.NewLocal(githubReleaseCreateReq.ProviderName, githubReleaseCreateReq.BinaryName)
	err := stagedRelease.Initialize()
	if err != nil {
		return err
	}

	defer stagedRelease.Close()

	lvl, err := zapcore.ParseLevel(rootCfg.logLevel)
	if err != nil {
		return err
	}

	err = stagedRelease.SetLogLevel(lvl)
	if err != nil {
		return err
	}

	err = stagedRelease.AddReleaseManifest(ctx, githubReleaseCreateReq.ManifestPath, githubReleaseCreateReq.Version)
	if err != nil {
		return err
	}

	err = stagedRelease.AddGoBinariesFrom(githubReleaseCreateReq.DistDir)
	if err != nil {
		return err
	}

	err = stagedRelease.CreateVersionedRegistryManifest(ctx)
	if err != nil {
		return err
	}

	err = stagedRelease.WriteSHA256Sums(ctx, githubReleaseCreateReq.GPGIdentityName, true)
	if err != nil {
		return err
	}

	return stagedRelease.PublishToGithubReleases(ctx, githubReleaseCreateReq)
}
