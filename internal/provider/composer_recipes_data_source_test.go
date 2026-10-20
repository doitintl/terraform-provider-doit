package provider_test

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

var (
	composerRecipeInfoOnce         sync.Once
	composerRecipeTotalCount       int
	composerRecipeWithResultsID    int64
	composerRecipeWithResultsCount int
	composerRecipeInfoErr          error
)

func initComposerRecipeTestInfo(t *testing.T) {
	t.Helper()
	composerRecipeInfoOnce.Do(func() {
		client := getAPIClient(t)
		resp, err := client.ListComposerRecipesWithResponse(t.Context(), &models.ListComposerRecipesParams{
			MaxResults: new(50),
		})
		if err != nil {
			composerRecipeInfoErr = fmt.Errorf("failed to list composer recipes: %w", err)
			return
		}
		if resp.JSON200 == nil {
			composerRecipeInfoErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode(), string(resp.Body))
			return
		}
		composerRecipeTotalCount = int(resp.JSON200.RowCount)
		for _, item := range resp.JSON200.Items {
			if item.ActiveResultsCount >= 2 {
				composerRecipeWithResultsID = item.Id
				composerRecipeWithResultsCount = item.ActiveResultsCount
				break
			}
			if composerRecipeWithResultsID == 0 && item.TotalResultsCount > 0 {
				composerRecipeWithResultsID = item.Id
				composerRecipeWithResultsCount = item.TotalResultsCount
			}
		}
	})
	if composerRecipeInfoErr != nil {
		t.Fatalf("Composer recipes pre-check failed: %v", composerRecipeInfoErr)
	}
}

func TestAccComposerRecipesDataSource_AutoPagination(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount == 0 {
		t.Skip("No Composer recipes in test account")
	}

	expectedCountStr := strconv.Itoa(composerRecipeTotalCount)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `data "doit_composer_recipes" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.test", "row_count", expectedCountStr),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.test", "items.#", expectedCountStr),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.title"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.provider"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.severity"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.test", "items.0.metadata.important_points.#"),
					resource.TestCheckNoResourceAttr("data.doit_composer_recipes.test", "page_token"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccComposerRecipesDataSource_MaxResultsOnly(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount < 2 {
		t.Skipf("Need at least 2 Composer recipes to test pagination, got %d", composerRecipeTotalCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "limited" {
  max_results = 1
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.limited", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.limited", "items.0.id"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.limited", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.limited", "page_token"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccComposerRecipesDataSource_PageTokenOnly(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount < 2 {
		t.Skipf("Need at least 2 Composer recipes to test page_token, got %d", composerRecipeTotalCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "first_page" {
  max_results = 1
}

data "doit_composer_recipes" "from_token" {
  page_token = data.doit_composer_recipes.first_page.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.first_page", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.first_page", "page_token"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.from_token", "row_count"),
					resource.TestCheckNoResourceAttr("data.doit_composer_recipes.from_token", "page_token"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_composer_recipes.first_page", "items.0.id",
						"data.doit_composer_recipes.from_token", "items.0.id",
					),
				),
			},
		},
	})
}

func TestAccComposerRecipesDataSource_MaxResultsAndPageToken(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeTotalCount < 2 {
		t.Skipf("Need at least 2 Composer recipes to test pagination, got %d", composerRecipeTotalCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "page1" {
  max_results = 1
}

data "doit_composer_recipes" "page2" {
  max_results = 1
  page_token  = data.doit_composer_recipes.page1.page_token
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.page1", "items.#", "1"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.page2", "items.#", "1"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_composer_recipes.page1", "items.0.id",
						"data.doit_composer_recipes.page2", "items.0.id",
					),
				),
			},
		},
	})
}

func TestAccComposerRecipesDataSource_FilterAndSort(t *testing.T) {
	initComposerRecipeTestInfo(t)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "filtered" {
  filter     = "has_active_results:true"
  sort_by    = "severity"
  sort_order = "desc"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.filtered", "filter", "has_active_results:true"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.filtered", "sort_by", "severity"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.filtered", "sort_order", "desc"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipes.filtered", "row_count"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccComposerRecipesDataSource_EmptyResult(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipes" "empty" {
  filter = "provider:vmware"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipes.empty", "row_count", "0"),
					resource.TestCheckResourceAttr("data.doit_composer_recipes.empty", "items.#", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}
