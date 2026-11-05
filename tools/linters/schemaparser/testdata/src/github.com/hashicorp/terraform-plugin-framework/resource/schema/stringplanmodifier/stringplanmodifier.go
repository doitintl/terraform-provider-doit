// Stub package for analysistest.
package stringplanmodifier

import "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"

func UseStateForUnknown() planmodifier.String { return nil }

func RequiresReplace() planmodifier.String { return nil }
