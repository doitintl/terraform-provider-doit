package provider_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

type readDataSourceCountingProviderServer struct {
	tfprotov6.ProviderServer
	readDataSourceCalls *atomic.Int64
}

func (s *readDataSourceCountingProviderServer) ReadDataSource(ctx context.Context, req *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	s.readDataSourceCalls.Add(1)

	return s.ProviderServer.ReadDataSource(ctx, req)
}

func TestAccDataSource_UnknownConfigDeferredByCore(t *testing.T) {
	var readDataSourceCalls atomic.Int64
	providerFactory := providerserver.NewProtocol6WithError(provider.New("dev")())
	providerFactories := map[string]func() (tfprotov6.ProviderServer, error){
		"doit": func() (tfprotov6.ProviderServer, error) {
			server, err := providerFactory()
			if err != nil {
				return nil, err
			}

			return &readDataSourceCountingProviderServer{
				ProviderServer:      server,
				readDataSourceCalls: &readDataSourceCalls,
			}, nil
		},
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		TerraformVersionChecks:   testAccTFVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: `
provider "doit" {
  api_token = "unused"
  host      = "http://127.0.0.1:1"
}

resource "terraform_data" "upstream" {}

data "doit_alerts" "test" {
  filter = terraform_data.upstream.id
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPreRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("data.doit_alerts.test", plancheck.ResourceActionRead),
						plancheck.ExpectUnknownValue("data.doit_alerts.test", tfjsonpath.New("alerts")),
						plancheck.ExpectUnknownValue("data.doit_alerts.test", tfjsonpath.New("row_count")),
						plancheck.ExpectUnknownValue("data.doit_alerts.test", tfjsonpath.New("page_token")),
					},
				},
			},
		},
	})

	if calls := readDataSourceCalls.Load(); calls != 0 {
		t.Errorf("expected Terraform Core to defer ReadDataSource, got %d call(s)", calls)
	}
}
