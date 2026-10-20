package provider_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestAccComposerRecipeResultsDataSource_AutoPagination(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeWithResultsID == 0 {
		t.Skip("No Composer recipe with results found in test account")
	}

	recipeIDStr := strconv.FormatInt(composerRecipeWithResultsID, 10)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "doit_composer_recipe_results" "test" {
  recipe_id = %d
}
`, composerRecipeWithResultsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.test", "recipe_id", recipeIDStr),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "row_count"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.id"),
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.test", "items.0.rule_id", recipeIDStr),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.account_id"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.issued_resource_id"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.issued_resource_name"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.provider"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.detection_time"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.is_active"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.test", "items.0.solved"),
					resource.TestCheckNoResourceAttr("data.doit_composer_recipe_results.test", "page_token"),
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

func TestAccComposerRecipeResultsDataSource_MaxResultsAndPageToken(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeWithResultsID == 0 || composerRecipeWithResultsCount < 2 {
		t.Skipf("Need a Composer recipe with at least 2 results to test pagination, got recipe %d with %d results", composerRecipeWithResultsID, composerRecipeWithResultsCount)
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "doit_composer_recipe_results" "page1" {
  recipe_id   = %d
  max_results = 1
}

data "doit_composer_recipe_results" "page2" {
  recipe_id   = %d
  max_results = 1
  page_token  = data.doit_composer_recipe_results.page1.page_token
}
`, composerRecipeWithResultsID, composerRecipeWithResultsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.page1", "items.#", "1"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.page1", "page_token"),
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.page2", "items.#", "1"),
					testCheckResourceAttrNotEqualAttr(
						"data.doit_composer_recipe_results.page1", "items.0.id",
						"data.doit_composer_recipe_results.page2", "items.0.id",
					),
				),
			},
		},
	})
}

func TestAccComposerRecipeResultsDataSource_FilterAndSort(t *testing.T) {
	initComposerRecipeTestInfo(t)
	if composerRecipeWithResultsID == 0 {
		t.Skip("No Composer recipe with results found in test account")
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "doit_composer_recipe_results" "filtered" {
  recipe_id  = %d
  filter     = "status:active"
  sort_by    = "detection_time"
  sort_order = "desc"
}
`, composerRecipeWithResultsID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.filtered", "filter", "status:active"),
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.filtered", "sort_by", "detection_time"),
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.filtered", "sort_order", "desc"),
					resource.TestCheckResourceAttrSet("data.doit_composer_recipe_results.filtered", "row_count"),
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

func TestAccComposerRecipeResultsDataSource_NonExistentRecipe(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProvidersProtoV6Factories,
		PreCheck:                 testAccPreCheckFunc(t),
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		Steps: []resource.TestStep{
			{
				Config: `
data "doit_composer_recipe_results" "nonexistent" {
  recipe_id = 999999999
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.nonexistent", "recipe_id", "999999999"),
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.nonexistent", "row_count", "0"),
					resource.TestCheckResourceAttr("data.doit_composer_recipe_results.nonexistent", "items.#", "0"),
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
