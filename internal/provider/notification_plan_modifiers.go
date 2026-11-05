package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// useNullForOmittedNotificationSetting proposes null for a notification
// settings object (a subscribed type of doit_user_notification, or the repeat
// controls of a cost-anomaly notification) when the configuration omits it.
// Omitting a type unsubscribes it, and omitting repeat, when_cost_grows or
// min_increase turns that resend control off; the update sends an explicit
// null for each. Planning null also on create and when the object is not
// stored keeps plans free of unknown values for the many types a notification
// does not subscribe to.
func useNullForOmittedNotificationSetting() planmodifier.Object {
	return useNullForOmittedNotificationSettingModifier{}
}

type useNullForOmittedNotificationSettingModifier struct{}

func (m useNullForOmittedNotificationSettingModifier) Description(_ context.Context) string {
	return "Proposes null when the notification setting is omitted from configuration, so omitting it unsubscribes or turns it off."
}

func (m useNullForOmittedNotificationSettingModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useNullForOmittedNotificationSettingModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() {
		resp.PlanValue = req.ConfigValue
	}
}

// planNotificationMetadataUnknownOnUpdate marks etag and update_time unknown
// when the plan updates the notification. The API changes both on every
// update, and the framework marks them unknown only when Terraform's proposed
// state differs from the prior state (fwserver/server_planresourcechange.go,
// v1.19.0). Removing a setting changes nothing in the proposed state, because
// Terraform proposes the stored value of an omitted Optional+Computed
// attribute; the clearing modifiers plan the update afterwards. Without this,
// state would keep the old etag and the next update without a refresh would
// fail with 412.
func planNotificationMetadataUnknownOnUpdate(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || resp.Plan.Raw.Equal(req.State.Raw) {
		return // create, destroy, or no change
	}
	for _, name := range []string{"etag", "update_time"} {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), types.StringUnknown())...)
	}
}
