package docker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/container"
	"github.com/docker/cli/cli/command/image"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/flant/werf/integration/utils"
)

var cli *command.DockerCli
var apiClient *client.Client

func init() {
	if err := initCli(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "init docker cli failed: %s\n", err)
		os.Exit(1)
	}

	if err := initApiClient(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "init docker api client failed: %s\n", err)
		os.Exit(1)
	}
}

func initCli() error {
	cliOpts := []command.DockerCliOption{
		command.WithContentTrust(false),
		command.WithOutputStream(GinkgoWriter),
		command.WithErrorStream(GinkgoWriter),
	}

	logrus.SetOutput(GinkgoWriter)

	newCli, err := command.NewDockerCli(cliOpts...)
	if err != nil {
		return err
	}

	opts := flags.NewClientOptions()
	if err := newCli.Initialize(opts); err != nil {
		return err
	}

	cli = newCli

	return nil
}

func initApiClient() error {
	ctx := context.Background()
	serverVersion, err := cli.Client().ServerVersion(ctx)
	if err != nil {
		return err
	}

	apiClient, err = client.NewClientWithOpts(client.WithVersion(serverVersion.APIVersion))
	if err != nil {
		return err
	}

	return nil
}

func ContainerStopAndRemove(containerName string) {
	Ω(CliStop(containerName)).Should(Succeed(), fmt.Sprintf("docker stop %s", containerName))
	Ω(CliRm(containerName)).Should(Succeed(), fmt.Sprintf("docker rm %s", containerName))
}

func ImageRemoveIfExists(imageName string) {
	_, err := imageInspect(imageName)
	if err == nil {
		Ω(CliRmi(imageName)).Should(Succeed(), "docker rmi")
	} else {
		if !strings.HasPrefix(err.Error(), "Error: No such image") {
			Ω(err).ShouldNot(HaveOccurred())
		}
	}
}

func ImageParent(imageName string) string {
	return ImageInspect(imageName).Parent
}

func ImageID(imageName string) string {
	return ImageInspect(imageName).ID
}

func ImageInspect(imageName string) *types.ImageInspect {
	inspect, err := imageInspect(imageName)
	Ω(err).ShouldNot(HaveOccurred())
	return inspect
}

func LocalDockerRegistryRun() (string, string) {
	containerName := fmt.Sprintf("werf_test_docker_registry-%s", utils.GetRandomString(10))
	imageName := "registry"

	hostPort := strconv.Itoa(utils.GetFreeTCPHostPort())
	dockerCliRunArgs := []string{
		"-d",
		"-p", fmt.Sprintf("%s:5000", hostPort),
		"-e", "REGISTRY_STORAGE_DELETE_ENABLED=true",
		"--name", containerName,
		imageName,
	}
	err := CliRun(dockerCliRunArgs...)
	Ω(err).ShouldNot(HaveOccurred(), "docker run "+strings.Join(dockerCliRunArgs, " "))

	registry := fmt.Sprintf("localhost:%s", hostPort)
	registryWithScheme := fmt.Sprintf("http://%s", registry)

	utils.WaitTillHostReadyToRespond(registryWithScheme, utils.DefaultWaitTillHostReadyToRespondMaxAttempts)

	return registry, containerName
}

func CliRun(args ...string) error {
	cmd := container.NewRunCommand(cli)
	return cmdExecute(cmd, args)
}

func CliRm(args ...string) error {
	cmd := container.NewRmCommand(cli)
	return cmdExecute(cmd, args)
}

func CliStop(args ...string) error {
	cmd := container.NewStopCommand(cli)
	return cmdExecute(cmd, args)
}

func CliPull(args ...string) error {
	cmd := image.NewPullCommand(cli)
	return cmdExecute(cmd, args)
}

func CliPush(args ...string) error {
	cmd := image.NewPushCommand(cli)
	return cmdExecute(cmd, args)
}

func CliTag(args ...string) error {
	cmd := image.NewTagCommand(cli)
	return cmdExecute(cmd, args)
}

func CliRmi(args ...string) error {
	cmd := image.NewRemoveCommand(cli)
	return cmdExecute(cmd, args)
}

func cmdExecute(cmd *cobra.Command, args []string) error {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	return cmd.Execute()
}

func Images(options types.ImageListOptions) ([]types.ImageSummary, error) {
	ctx := context.Background()
	images, err := apiClient.ImageList(ctx, options)
	if err != nil {
		return nil, err
	}

	return images, nil
}

func imageInspect(ref string) (*types.ImageInspect, error) {
	ctx := context.Background()
	inspect, _, err := apiClient.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return nil, err
	}

	return &inspect, nil
}
