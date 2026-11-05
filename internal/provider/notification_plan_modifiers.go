package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
