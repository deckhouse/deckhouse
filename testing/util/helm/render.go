package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

type Renderer struct {
	Name      string
	Namespace string
	LintMode  bool
}

func (r Renderer) RenderChartFromDir(dir string, values string) (files map[string]string, err error) {
	c, err := loader.Load(dir)
	if err != nil {
		panic(fmt.Errorf("chart load from '%s': %v", dir, err))
	}
	return r.RenderChart(c, values)
}

func (r Renderer) RenderChart(c *chart.Chart, values string) (files map[string]string, err error) {
	// prepare values
	vals, err := chartutil.ReadValues([]byte(values))
	if err != nil {
		return nil, fmt.Errorf("helm chart read raw values: %v", err)
	}

	releaseName := "release"
	if r.Name != "" {
		releaseName = r.Name
	}
	releaseNamespace := "default"
	if r.Namespace != "" {
		releaseNamespace = r.Namespace
	}
	releaseOptions := chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: releaseNamespace,
		IsInstall: true,
		IsUpgrade: true,
	}

	// TODO is it needed here for tests?
	cvals, err := chartutil.CoalesceValues(c, vals.AsMap())
	if err != nil {
		return nil, fmt.Errorf("helm chart coalesce values: %v", err)
	}

	valuesToRender, err := chartutil.ToRenderValues(c, cvals, releaseOptions, nil)
	if err != nil {
		return nil, fmt.Errorf("helm chart prepare render values: %v", err)
	}

	// render chart with prepared values
	var e engine.Engine
	e.Strict = false
	e.LintMode = r.LintMode
	out, err := e.Render(c, valuesToRender)
	if err != nil {
		return nil, fmt.Errorf("helm chart render: %v", err)
	}

	return out, nil
}
