package provider

import (
	"context"
	"fmt"
	"math/big"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Helpers shared by the notification resources that carry cost-anomaly
// settings (doit_user_notification, doit_customer_notification).

// costAnomalyAmountPattern is a whole number of dollars, optionally with
// zero cents. The API rejects fractional amounts and more than two decimals,
// and echoes accepted amounts with two decimals ("100" → "100.00").
var costAnomalyAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.0{1,2})?$`)

const costAnomalyAmountMessage = "must be a whole number of US dollars, such as `100` or `100.00`"

// preserveCostAnomalyAmount keeps the amount in state when it has the same
// value as the API's. The API always answers with two decimals ("100" →
// "100.00"), so taking its spelling would drift from a config that omits them.
func preserveCostAnomalyAmount(prior types.String, apiAmount string) types.String {
	if !prior.IsUnknown() && !prior.IsNull() && costAnomalyAmountsEqual(prior.ValueString(), apiAmount) {
		return prior
	}
	return types.StringValue(apiAmount)
}

func costAnomalyAmountsEqual(a, b string) bool {
	ra, ok := new(big.Rat).SetString(a)
	if !ok {
		return false
	}
	rb, ok := new(big.Rat).SetString(b)
	if !ok {
		return false
	}
	return ra.Cmp(rb) == 0
}

// costAnomalyMaxAmount is the largest amount the API accepts, in US dollars.
var costAnomalyMaxAmount = big.NewRat(1_000_000_000, 1)

// costAnomalyAmountRangeValidator enforces the API's bounds on a Money
// amount. Malformed amounts are left to the pattern validator.
type costAnomalyAmountRangeValidator struct {
	// positive rejects zero (min_increase); min_excess_cost accepts it.
	positive bool
}

var _ validator.String = costAnomalyAmountRangeValidator{}

func (v costAnomalyAmountRangeValidator) Description(_ context.Context) string {
	if v.positive {
		return "amount must be greater than 0 and at most 1000000000 US dollars"
	}
	return "amount must be at most 1000000000 US dollars"
}

func (v costAnomalyAmountRangeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v costAnomalyAmountRangeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	amount, ok := new(big.Rat).SetString(req.ConfigValue.ValueString())
	if !ok {
		return
	}

	if amount.Cmp(costAnomalyMaxAmount) > 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Amount",
			fmt.Sprintf("Attribute %s must be at most 1000000000 US dollars, got: %s", req.Path, req.ConfigValue.ValueString()),
		)
	}
	if v.positive && amount.Sign() <= 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Amount",
			fmt.Sprintf("Attribute %s must be greater than 0, got: %s", req.Path, req.ConfigValue.ValueString()),
		)
	}
}

const costAnomalyAmountDescription = "Amount in whole US dollars, such as `100` or `100.00`. " +
	"The API returns amounts with two decimals; state keeps the configured spelling while the value is unchanged, " +
	"but an imported notification has the two-decimal spelling."

// setCostAnomalyDescriptions replaces the descriptions of the named attributes.
func setCostAnomalyDescriptions(attributes map[string]schema.Attribute, descriptions map[string]string) {
	for name, description := range descriptions {
		switch a := attributes[name].(type) {
		case schema.StringAttribute:
			a.Description, a.MarkdownDescription = description, description
			attributes[name] = a
		case schema.Int64Attribute:
			a.Description, a.MarkdownDescription = description, description
			attributes[name] = a
		case schema.ListAttribute:
			a.Description, a.MarkdownDescription = description, description
			attributes[name] = a
		case schema.SingleNestedAttribute:
			a.Description, a.MarkdownDescription = description, description
			attributes[name] = a
		default:
			panic(fmt.Sprintf("setCostAnomalyDescriptions: unsupported attribute %q (%T)", name, a))
		}
	}
}

// appendCostAnomalyMoneyValidators tightens the generated Money checks to
// what the API accepts: whole US dollars up to 1,000,000,000, and above zero
// when positive is set.
func appendCostAnomalyMoneyValidators(attributes map[string]schema.Attribute, positive bool) {
	if amount, ok := attributes["amount"].(schema.StringAttribute); ok {
		amount.Validators = append(amount.Validators,
			stringvalidator.RegexMatches(costAnomalyAmountPattern, costAnomalyAmountMessage),
			costAnomalyAmountRangeValidator{positive: positive},
		)
		attributes["amount"] = amount
	}
	if currency, ok := attributes["currency"].(schema.StringAttribute); ok {
		currency.Validators = append(currency.Validators, stringvalidator.OneOf("USD"))
		currency.Description = "Currency code. Must be `USD`."
		currency.MarkdownDescription = currency.Description
		attributes["currency"] = currency
	}
}
