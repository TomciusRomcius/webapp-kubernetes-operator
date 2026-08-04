package helm

import (
	"errors"
	"fmt"
	"os"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	"helm.sh/helm/v4/pkg/storage/driver"
)

type HelmClient struct {
}

type ChartActionConfig struct {
	ReleaseName string
	Namespace   string
	ChartName   string
	RepoURL     string
	Version     string
}

func (h *HelmClient) newActionConfig(namespace string) (*action.Configuration, *cli.EnvSettings, error) {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return nil, nil, fmt.Errorf("failed to init helm action config: %w", err)
	}
	return actionConfig, settings, nil
}

func (h *HelmClient) chartVersion(chrt any) string {
	switch c := chrt.(type) {
	case *chartv2.Chart:
		if c != nil && c.Metadata != nil {
			return c.Metadata.Version
		}
	case chartv2.Chart:
		if c.Metadata != nil {
			return c.Metadata.Version
		}
	}
	return ""
}

func (h *HelmClient) InstallOrUpgradeChart(cfg *ChartActionConfig, values map[string]any) (release.Releaser, error) {
	actionConfig, settings, err := h.newActionConfig(cfg.Namespace)
	if err != nil {
		return nil, err
	}

	chartPathOptions := action.ChartPathOptions{
		RepoURL: cfg.RepoURL,
		Version: cfg.Version,
	}
	chartPath, err := chartPathOptions.LocateChart(cfg.ChartName, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to locate chart %q: %w", cfg.ChartName, err)
	}

	chrt, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart %q: %w", chartPath, err)
	}

	desiredVersion := h.chartVersion(chrt)

	getAction := action.NewGet(actionConfig)
	currentRel, err := getAction.Run(cfg.ReleaseName)
	needsInstall := errors.Is(err, driver.ErrReleaseNotFound)
	var current release.Accessor
	if err == nil {
		current, err = release.NewAccessor(currentRel)
		if err != nil {
			return nil, fmt.Errorf("failed to read release %q: %w", cfg.ReleaseName, err)
		}
		needsInstall = current.Status() == common.StatusUninstalled.String()
	} else if !needsInstall {
		return nil, fmt.Errorf("failed to get release %q: %w", cfg.ReleaseName, err)
	}

	if needsInstall {
		installAction := action.NewInstall(actionConfig)
		installAction.ChartPathOptions = chartPathOptions
		installAction.WaitStrategy = "watcher"
		installAction.ReleaseName = cfg.ReleaseName
		installAction.Namespace = cfg.Namespace
		installAction.CreateNamespace = true
		if current != nil && current.Status() == common.StatusUninstalled.String() {
			installAction.Replace = true
		}
		rel, err := installAction.Run(chrt, values)
		if err != nil {
			return nil, fmt.Errorf("failed to install release %q: %w", cfg.ReleaseName, err)
		}
		return rel, nil
	}

	installedVersion := h.chartVersion(current.Chart())

	// Already on the desired chart version and healthy: no-op.
	if current.Status() == common.StatusDeployed.String() && installedVersion == desiredVersion {
		return currentRel, nil
	}

	upgradeAction := action.NewUpgrade(actionConfig)
	upgradeAction.ChartPathOptions = chartPathOptions
	upgradeAction.Namespace = cfg.Namespace
	rel, err := upgradeAction.Run(cfg.ReleaseName, chrt, values)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade release %q: %w", cfg.ReleaseName, err)
	}
	return rel, nil
}
