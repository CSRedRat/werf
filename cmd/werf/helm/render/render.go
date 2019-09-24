package render

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/flant/logboek"

	"github.com/flant/werf/cmd/werf/common"
	helm_common "github.com/flant/werf/cmd/werf/helm/common"
	"github.com/flant/werf/pkg/deploy"
	"github.com/flant/werf/pkg/deploy/helm"
	"github.com/flant/werf/pkg/docker"
	"github.com/flant/werf/pkg/lock"
	"github.com/flant/werf/pkg/tmp_manager"
	"github.com/flant/werf/pkg/true_git"
	"github.com/flant/werf/pkg/werf"
)

var commonCmdData common.CmdData

func NewCmd() *cobra.Command {
	var outputFilePath string

	cmd := &cobra.Command{
		Use:                   "render",
		Short:                 "Render Werf chart templates to stdout",
		DisableFlagsInUseLine: true,
		Annotations: map[string]string{
			common.CmdEnvAnno: common.EnvsDescription(common.WerfSecretKey),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRender(outputFilePath)
		},
	}

	common.SetupDir(&commonCmdData, cmd)
	common.SetupTmpDir(&commonCmdData, cmd)
	common.SetupHomeDir(&commonCmdData, cmd)

	common.SetupNamespace(&commonCmdData, cmd)
	common.SetupRelease(&commonCmdData, cmd)
	common.SetupEnvironment(&commonCmdData, cmd)
	common.SetupDockerConfig(&commonCmdData, cmd, "")
	common.SetupAddAnnotations(&commonCmdData, cmd)
	common.SetupAddLabels(&commonCmdData, cmd)

	common.SetupSet(&commonCmdData, cmd)
	common.SetupSetString(&commonCmdData, cmd)
	common.SetupValues(&commonCmdData, cmd)
	common.SetupSecretValues(&commonCmdData, cmd)
	common.SetupIgnoreSecretKey(&commonCmdData, cmd)

	common.SetupImagesRepo(&commonCmdData, cmd)
	common.SetupImagesRepoMode(&commonCmdData, cmd)
	common.SetupTag(&commonCmdData, cmd)

	cmd.Flags().StringVarP(&outputFilePath, "output-file-path", "o", "", "Write to file instead of stdout")

	return cmd
}

func runRender(outputFilePath string) error {
	tmp_manager.AutoGCEnabled = false

	if err := werf.Init(*commonCmdData.TmpDir, *commonCmdData.HomeDir); err != nil {
		return fmt.Errorf("initialization error: %s", err)
	}

	if err := lock.Init(); err != nil {
		return err
	}

	if err := true_git.Init(true_git.Options{Out: logboek.GetOutStream(), Err: logboek.GetErrStream()}); err != nil {
		return err
	}

	if err := deploy.Init(deploy.InitOptions{HelmInitOptions: helm.InitOptions{WithoutKube: true}}); err != nil {
		return err
	}

	if err := docker.Init(*commonCmdData.DockerConfig); err != nil {
		return err
	}

	projectDir, err := common.GetProjectDir(&commonCmdData)
	if err != nil {
		return fmt.Errorf("getting project dir failed: %s", err)
	}

	werfConfig, err := common.GetWerfConfig(projectDir)
	if err != nil {
		return fmt.Errorf("bad config: %s", err)
	}

	optionalImagesRepo, err := common.GetOptionalImagesRepo(werfConfig.Meta.Project, &commonCmdData)
	if err != nil {
		return err
	}

	withoutImagesRepo := true
	if optionalImagesRepo != "" {
		withoutImagesRepo = false
	}

	imagesRepo := helm_common.GetImagesRepoOrStub(optionalImagesRepo)

	imagesRepoMode, err := common.GetImagesRepoMode(&commonCmdData)
	if err != nil {
		return err
	}

	imagesRepoManager, err := common.GetImagesRepoManager(imagesRepo, imagesRepoMode)
	if err != nil {
		return err
	}

	env := helm_common.GetEnvironmentOrStub(*commonCmdData.Environment)

	release, err := common.GetHelmRelease(*commonCmdData.Release, env, werfConfig)
	if err != nil {
		return err
	}

	namespace, err := common.GetKubernetesNamespace(*commonCmdData.Namespace, env, werfConfig)
	if err != nil {
		return err
	}

	tag, tagStrategy, err := helm_common.GetTagOrStub(&commonCmdData)
	if err != nil {
		return err
	}

	userExtraAnnotations, err := common.GetUserExtraAnnotations(&commonCmdData)
	if err != nil {
		return err
	}

	userExtraLabels, err := common.GetUserExtraLabels(&commonCmdData)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer([]byte{})
	if err := deploy.RunRender(buf, projectDir, werfConfig, deploy.RenderOptions{
		ReleaseName:          release,
		Tag:                  tag,
		TagStrategy:          tagStrategy,
		Namespace:            namespace,
		ImagesRepoManager:    imagesRepoManager,
		WithoutImagesRepo:    withoutImagesRepo,
		Values:               *commonCmdData.Values,
		SecretValues:         *commonCmdData.SecretValues,
		Set:                  *commonCmdData.Set,
		SetString:            *commonCmdData.SetString,
		Env:                  env,
		UserExtraAnnotations: userExtraAnnotations,
		UserExtraLabels:      userExtraLabels,
		IgnoreSecretKey:      *commonCmdData.IgnoreSecretKey,
	}); err != nil {
		return err
	}

	if outputFilePath != "" {
		if err := saveRenderedChart(outputFilePath, buf); err != nil {
			return err
		}
	} else {
		fmt.Printf(buf.String())
	}

	return nil
}

func saveRenderedChart(outputFilePath string, buf *bytes.Buffer) error {
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0777); err != nil {
		return err
	}

	if err := ioutil.WriteFile(outputFilePath, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
