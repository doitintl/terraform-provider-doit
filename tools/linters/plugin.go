// Package linters registers the custom linters as a golangci-lint module plugin.
package linters

import (
	"github.com/doitintl/terraform-provider-doit/tools/linters/clearableattr"
	"github.com/doitintl/terraform-provider-doit/tools/linters/configuretype"
	"github.com/doitintl/terraform-provider-doit/tools/linters/constructor"
	"github.com/doitintl/terraform-provider-doit/tools/linters/crudnaming"
	"github.com/doitintl/terraform-provider-doit/tools/linters/defaultdrift"
	"github.com/doitintl/terraform-provider-doit/tools/linters/delete404"
	"github.com/doitintl/terraform-provider-doit/tools/linters/diagdrop"
	"github.com/doitintl/terraform-provider-doit/tools/linters/diagsuppressed"
	"github.com/doitintl/terraform-provider-doit/tools/linters/errformat"
	"github.com/doitintl/terraform-provider-doit/tools/linters/interfacestyle"
	"github.com/doitintl/terraform-provider-doit/tools/linters/listnullread"
	"github.com/doitintl/terraform-provider-doit/tools/linters/methodreceiver"
	"github.com/doitintl/terraform-provider-doit/tools/linters/newexpr"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlaycheck"
	"github.com/doitintl/terraform-provider-doit/tools/linters/overlayinvariant"
	"github.com/doitintl/terraform-provider-doit/tools/linters/paralleltest"
	"github.com/doitintl/terraform-provider-doit/tools/linters/read404"
	"github.com/doitintl/terraform-provider-doit/tools/linters/requestguard"
	"github.com/doitintl/terraform-provider-doit/tools/linters/structliteral"
	"github.com/doitintl/terraform-provider-doit/tools/linters/timeoutcheck"
	"github.com/doitintl/terraform-provider-doit/tools/linters/unknownguard"
	"github.com/doitintl/terraform-provider-doit/tools/linters/usestatefunknown"
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/modernize"
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

	register.Plugin("unknownguard", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{unknownguard.Analyzer}}, nil
	})

	register.Plugin("diagdrop", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{diagdrop.Analyzer}}, nil
	})

	register.Plugin("defaultdrift", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{defaultdrift.Analyzer}}, nil
	})

	register.Plugin("methodreceiver", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{methodreceiver.Analyzer}}, nil
	})

	register.Plugin("requestguard", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{requestguard.Analyzer}}, nil
	})

	register.Plugin("clearableattr", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: []*analysis.Analyzer{clearableattr.Analyzer}}, nil
	})

	// modernize wraps the full x/tools modernize suite. Its "newexpr"
	// analyzer shares a Name with our custom newexpr linter but targets a
	// disjoint pattern (pointer-wrapper functions vs. temp-var-then-address),
	// and both run fine side by side under separate golangci plugins.
	register.Plugin("modernize", func(_ any) (register.LinterPlugin, error) {
		return &analyzerPlugin{analyzers: modernize.Suite}, nil
	})
}
