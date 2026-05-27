package paralleltest

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// BAD: uses resource.Test instead of resource.ParallelTest
func TestAccBad(t *testing.T) {
	resource.Test(t, resource.TestCase{}) // want `use resource.ParallelTest`
}

// GOOD: uses resource.ParallelTest
func TestAccGood(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{})
}
