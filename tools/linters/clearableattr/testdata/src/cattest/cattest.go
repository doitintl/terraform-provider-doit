package cattest

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// --- BAD: Schema() with unclassified Optional+Computed fields ---

type badResource struct{}

func (r *badResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := BadResourceSchema(ctx)

	// id gets UseStateForUnknown — Computed-only, not relevant to this linter.
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}

	// Each unclassified O+C leaf is referenced in its own override block (which
	// adds no modifier and no acknowledgeNotClearable), so it is flagged at a
	// deterministic line regardless of the fallback-position algorithm.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "description" has no clearable classification`
		s.Attributes["description"] = attr
	}
	if attr, ok := s.Attributes["folder_id"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "folder_id" has no clearable classification`
		s.Attributes["folder_id"] = attr
	}
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "public" has no clearable classification`
		s.Attributes["public"] = attr
	}
	if configAttr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if attr, ok := configAttr.Attributes["currency"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "config.currency" has no clearable classification`
			configAttr.Attributes["currency"] = attr
		}
		s.Attributes["config"] = configAttr
	}
	if attr, ok := s.Attributes["results"].(schema.ListNestedAttribute); ok {
		if nested, ok := attr.NestedObject.Attributes["external_id"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "results\[\*\].external_id" has no clearable classification`
			attr.NestedObject.Attributes["external_id"] = nested
		}
		if nested, ok := attr.NestedObject.Attributes["external_url"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "results\[\*\].external_url" has no clearable classification`
			attr.NestedObject.Attributes["external_url"] = nested
		}
		if nested, ok := attr.NestedObject.Attributes["metadata"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "results\[\*\].metadata" has no clearable classification`
			attr.NestedObject.Attributes["metadata"] = nested
		}
		s.Attributes["results"] = attr
	}

	resp.Schema = s
}

// --- GOOD: Schema() with all O+C fields classified via modifier ---

type goodResource struct{}

func (r *goodResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := GoodResourceSchema(ctx)

	// description: clearable.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// folder_id: clearable.
	if attr, ok := s.Attributes["folder_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["folder_id"] = attr
	}

	// public: clearable + UseStateForUnknown.
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			useEmptyForUnknownWhenConfigNull(),
			stringplanmodifier.UseStateForUnknown(),
		)
		s.Attributes["public"] = attr
	}

	// config.currency: clearable via direct nested modifier.
	if attr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if nested, ok := attr.Attributes["currency"].(schema.StringAttribute); ok {
			nested.PlanModifiers = append(nested.PlanModifiers, useEmptyForUnknownWhenConfigNull())
			attr.Attributes["currency"] = nested
		}
		s.Attributes["config"] = attr
	}

	// results[*].external_id, external_url, metadata: clearable via loop.
	if attr, ok := s.Attributes["results"].(schema.ListNestedAttribute); ok {
		for _, field := range []string{"external_id", "external_url", "metadata"} {
			if nested, ok := attr.NestedObject.Attributes[field].(schema.StringAttribute); ok {
				nested.PlanModifiers = append(nested.PlanModifiers, useEmptyForUnknownWhenConfigNull())
				attr.NestedObject.Attributes[field] = nested
			}
		}
		s.Attributes["results"] = attr
	}

	resp.Schema = s
}

// --- GOOD: Schema() where O+C attribute is overridden to Required ---

type overrideResource struct{}

func (r *overrideResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := OverrideResourceSchema(ctx)

	// description: overridden from O+C to Required — no longer needs classification.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
		s.Attributes["description"] = attr
	}

	// folder_id: overridden from O+C to Required.
	if attr, ok := s.Attributes["folder_id"].(schema.StringAttribute); ok {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
		s.Attributes["folder_id"] = attr
	}

	// public: overridden from O+C to Optional (no Computed).
	if attr, ok := s.Attributes["public"].(schema.StringAttribute); ok {
		attr.Optional = true
		attr.Computed = false
		s.Attributes["public"] = attr
	}

	// config: delete to avoid nested findings.
	delete(s.Attributes, "config")
	delete(s.Attributes, "results")

	resp.Schema = s
}

// --- GOOD: Schema() with Cat B attributes acknowledged via allowlist ---

type acknowledgedResource struct{}

func (r *acknowledgedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := AcknowledgedResourceSchema(ctx)

	// Cat A: clearable via modifier.
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, useEmptyForUnknownWhenConfigNull())
		s.Attributes["description"] = attr
	}

	// Cat B: acknowledged as not clearable — replaces no-op if-blocks.
	acknowledgeNotClearable(s,
		"folder_id",               // API-assigned
		"public",                  // API defaults to false
		"config.currency",         // API defaults to org currency
		"results[*].external_id",  // API-assigned identity
		"results[*].external_url", // API-assigned
		"results[*].metadata",     // API-assigned
	)

	resp.Schema = s
}

// --- GOOD: nested O+C object, Category A (modifier + explicit null-send) ---

type catANullSendResource struct{}

func (r *catANullSendResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := CatANullSendResourceSchema(ctx)

	if configAttr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if fsAttr, ok := configAttr.Attributes["forecast_settings"].(schema.SingleNestedAttribute); ok {
			fsAttr.PlanModifiers = append(fsAttr.PlanModifiers, useNullOrDefaultForForecastSettings())
			configAttr.Attributes["forecast_settings"] = fsAttr
		}
		s.Attributes["config"] = configAttr
	}

	resp.Schema = s
}

// toRequest sends the explicit null that makes forecast_settings clearable.
func (r *catANullSendResource) toRequest() {
	var externalConfig extConfig
	externalConfig.ForecastSettings.SetNull()
}

// --- BAD: nested O+C object, Category A modifier but no explicit null-send ---

type catAMissingNullResource struct{}

func (r *catAMissingNullResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := CatAMissingNullResourceSchema(ctx)

	if configAttr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if bsAttr, ok := configAttr.Attributes["budget_settings"].(schema.SingleNestedAttribute); ok { // want `clearable nested attribute .* must send an explicit null`
			bsAttr.PlanModifiers = append(bsAttr.PlanModifiers, useNullOrDefaultForForecastSettings())
			configAttr.Attributes["budget_settings"] = bsAttr
		}
		s.Attributes["config"] = configAttr
	}

	resp.Schema = s
}

// --- GOOD: nested O+C object, Category C (replace-on-clear via ModifyPlan) ---

type catCResource struct{}

func (r *catCResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := CatCResourceSchema(ctx)
	resp.Schema = s
}

func (r *catCResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	requiresReplaceWhenCleared[countValue](ctx, req, resp, path.Root("config").AtName("count"))
}

// --- BAD: nested O+C object with neither Category A nor Category C ---

type unclassifiedResource struct{}

func (r *unclassifiedResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := UnclassifiedResourceSchema(ctx)

	if configAttr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
		if thresholdAttr, ok := configAttr.Attributes["threshold"].(schema.SingleNestedAttribute); ok { // want `nested Optional\+Computed attribute .* has no clearable classification`
			configAttr.Attributes["threshold"] = thresholdAttr
		}
		s.Attributes["config"] = configAttr
	}

	resp.Schema = s
}

// --- GOOD: nested O+C object classified Category B via acknowledgeNotClearable ---

type catBContainerResource struct{}

func (r *catBContainerResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := CatBContainerResourceSchema(ctx)

	// config.settings is a nested single-nested object that is silently preserved
	// (no drift), so acknowledging it as not-clearable is sufficient — no object
	// plan modifier or requiresReplaceWhenCleared needed.
	acknowledgeNotClearable(s, "config.settings")

	resp.Schema = s
}

// --- GOOD/BAD: single-nested object inside a list element ---

type listElementResource struct{}

func (r *listElementResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := ListElementResourceSchema(ctx)

	// items[*].detail is a single-nested object inside a list element: exempt from
	// the container check (the list is the clearable unit), so it is NOT flagged.
	// Its O+C leaf items[*].detail.note is still flagged.
	if itemsAttr, ok := s.Attributes["items"].(schema.ListNestedAttribute); ok {
		if detailAttr, ok := itemsAttr.NestedObject.Attributes["detail"].(schema.SingleNestedAttribute); ok {
			if note, ok := detailAttr.Attributes["note"].(schema.StringAttribute); ok { // want `Optional\+Computed attribute "items\[\*\].detail.note" has no clearable classification`
				detailAttr.Attributes["note"] = note
			}
			itemsAttr.NestedObject.Attributes["detail"] = detailAttr
		}
		s.Attributes["items"] = itemsAttr
	}

	resp.Schema = s
}

// useEmptyForUnknownWhenConfigNull is a stub for the plan modifier.
func useEmptyForUnknownWhenConfigNull() stringplanmodifier.PlanModifier {
	return stringplanmodifier.PlanModifier{}
}

// acknowledgeNotClearable is a stub for the runtime helper.
func acknowledgeNotClearable(_ schema.Schema, _ ...string) {}

// useNullOrDefaultForForecastSettings is a stub for a nested-object clearing plan
// modifier (does not follow the use…WhenConfigNull naming convention).
func useNullOrDefaultForForecastSettings() objectPlanModifier { return objectPlanModifier{} }

type objectPlanModifier struct{}

// countValue stands in for a generated nested custom value type.
type countValue struct{}

// requiresReplaceWhenCleared is a stub for the Category C ModifyPlan helper.
func requiresReplaceWhenCleared[T any](_ context.Context, _ resource.ModifyPlanRequest, _ *resource.ModifyPlanResponse, _ path.Path) {
}

// nullableObj/extConfig stub the request-builder side; SetNull sends explicit null.
type nullableObj struct{}

func (nullableObj) SetNull() {}

type extConfig struct {
	ForecastSettings nullableObj
}
