// Package linters registers the custom linters as a golangci-lint module plugin.
package linters

import (
	"github.com/doitintl/terraform-provider-doit/tools/linters/diagsuppressed"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlaycheck"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlayinvariant"
	"github.com/doitintl/terraform-provider-doit/tools/linters/structliteral"
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

type analyzerPlugin struct {
	analyzers []*analysis.Analyzer
}

func (p *analyzerPlugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return p.analyzers, nil
}

func (p *analyzerPlugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

func init() {
	register.Plugin("diagsuppressed", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{diagsuppressed.Analyzer}}, nil
	})

	register.Plugin("structliteral", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{structliteral.Analyzer}}, nil
	})

	register.Plugin("overlayinvariant", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{overlayinvariant.Analyzer}}, nil
	})

	register.Plugin("overlaycheck", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{overlaycheck.Analyzer}}, nil
	})
}
