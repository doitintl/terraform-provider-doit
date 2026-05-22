// Package linters registers the custom linters as a golangci-lint module plugin.
package linters

import (
	"github.com/doitintl/terraform-provider-doit/tools/linters/configuretype"
	"github.com/doitintl/terraform-provider-doit/tools/linters/constructor"
	"github.com/doitintl/terraform-provider-doit/tools/linters/crudnaming"
	"github.com/doitintl/terraform-provider-doit/tools/linters/delete404"
	"github.com/doitintl/terraform-provider-doit/tools/linters/diagsuppressed"
	"github.com/doitintl/terraform-provider-doit/tools/linters/errformat"
	"github.com/doitintl/terraform-provider-doit/tools/linters/interfacestyle"
	"github.com/doitintl/terraform-provider-doit/tools/linters/listnullread"
	"github.com/doitintl/terraform-provider-doit/tools/linters/newexpr"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlaycheck"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlayinvariant"
	"github.com/doitintl/terraform-provider-doit/tools/linters/paralleltest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/read404"
	"github.com/doitintl/terraform-provider-doit/tools/linters/structliteral"
	"github.com/doitintl/terraform-provider-doit/tools/linters/timeoutcheck"
	"github.com/doitintl/terraform-provider-doit/tools/linters/usestatefunknown"
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

	register.Plugin("delete404", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{delete404.Analyzer}}, nil
	})

	register.Plugin("timeoutcheck", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{timeoutcheck.Analyzer}}, nil
	})

	register.Plugin("read404", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{read404.Analyzer}}, nil
	})

	register.Plugin("configuretype", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{configuretype.Analyzer}}, nil
	})

	register.Plugin("errformat", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{errformat.Analyzer}}, nil
	})

	register.Plugin("listnullread", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{listnullread.Analyzer}}, nil
	})

	register.Plugin("usestatefunknown", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{usestatefunknown.Analyzer}}, nil
	})

	register.Plugin("paralleltest", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{paralleltest.Analyzer}}, nil
	})

	register.Plugin("interfacestyle", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{interfacestyle.Analyzer}}, nil
	})

	register.Plugin("crudnaming", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{crudnaming.Analyzer}}, nil
	})

	register.Plugin("constructor", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{constructor.Analyzer}}, nil
	})

	register.Plugin("newexpr", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{newexpr.Analyzer}}, nil
	})
}

