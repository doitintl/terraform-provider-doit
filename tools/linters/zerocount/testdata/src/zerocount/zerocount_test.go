package zerocount

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const listKey = "items.#"

func TestBad(t *testing.T) {
	_ = resource.TestCheckResourceAttr("r.a", "items.#", "0")      // want `also passes when the attribute is null`
	_ = resource.TestCheckResourceAttr("r.a", "settings.x.%", "0") // want `empty map or object`
	_ = resource.TestCheckResourceAttr("r.a", listKey, "0")        // want `empty list or set`
	_ = resource.TestCheckResourceAttr("r.a", "a."+"b.#", "0")     // want `a\.b\.#`
}

func TestGood(t *testing.T) {
	_ = resource.TestCheckResourceAttr("r.a", "items.#", "1")
	_ = resource.TestCheckResourceAttr("r.a", "settings.x.%", "2")
	_ = resource.TestCheckResourceAttr("r.a", "name", "0")
	_ = resource.TestCheckResourceAttr("r.a", "items.0", "0")
	_ = resource.TestCheckResourceAttrSet("r.a", "items.#")

	n := "0"
	_ = resource.TestCheckResourceAttr("r.a", "items.#", n)
}
