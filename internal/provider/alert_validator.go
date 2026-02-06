package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// alertRecipientsValidator validates that alerts have at least one recipient.
// The API adds the creator as a default recipient when recipients is empty,
// but this causes state drift. Requiring at least one recipient avoids this issue.
type alertRecipientsValidator struct{}

func (v alertRecipientsValidator) Description(_ context.Context) string {
	return "Validates that alerts have at least one recipient"
}

func (v alertRecipientsValidator) MarkdownDescription(_ context.Context) string {
	return "Validates that `recipients` has at least one entry"
}

func (v alertRecipientsValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var recipients types.List

	// Get the recipients attribute
	diags := req.Config.GetAttribute(ctx, path.Root("recipients"), &recipients)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If null/unknown, skip validation - let schema handle required vs optional
	if recipients.IsNull() || recipients.IsUnknown() {
		return
	}

	// Validate that there's at least one recipient
	if len(recipients.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("recipients"),
			"At Least One Recipient Required",
			"The 'recipients' attribute must contain at least one email address. "+
				"If you want to use the default (creator), omit the recipients attribute entirely.",
		)
	}
}
