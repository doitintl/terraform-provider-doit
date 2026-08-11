package provider

import (
	"context"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

// The API omits config.timeRange.unit for a custom range, whose bounds come
// from customTimeRange instead. Both mappers must render an omitted mode or
// unit as null rather than dereferencing it.
func TestMapReportToModel_TimeRangeNilModeAndUnit(t *testing.T) {
	ctx := context.Background()

	customMode := models.TimeSettingsMode("custom")
	amount := int64(0)

	tests := []struct {
		name         string
		timeRange    *models.TimeSettings
		wantModeNull bool
		wantUnitNull bool
		wantMode     string
	}{
		{
			name:         "unit omitted for a custom range",
			timeRange:    &models.TimeSettings{Mode: &customMode, Amount: &amount},
			wantUnitNull: true,
			wantMode:     "custom",
		},
		{
			name:         "mode and unit both omitted",
			timeRange:    &models.TimeSettings{Amount: &amount},
			wantModeNull: true,
			wantUnitNull: true,
		},
		{
			name: "both populated",
			timeRange: &models.TimeSettings{
				Mode:   &customMode,
				Amount: &amount,
				Unit:   func() *models.TimeSettingsUnit { u := models.TimeSettingsUnit("day"); return &u }(),
			},
			wantMode: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &models.ExternalReport{
				Config: &models.ExternalConfig{TimeRange: tt.timeRange},
			}

			// Resource mapper.
			state := &reportResourceModel{}
			if diags := mapReportToModel(ctx, resp, state); diags.HasError() {
				t.Fatalf("mapReportToModel returned errors: %v", diags)
			}

			tr := state.Config.TimeRange

			if got := tr.Unit.IsNull(); got != tt.wantUnitNull {
				t.Errorf("resource unit IsNull() = %v, want %v", got, tt.wantUnitNull)
			}

			if got := tr.Mode.IsNull(); got != tt.wantModeNull {
				t.Errorf("resource mode IsNull() = %v, want %v", got, tt.wantModeNull)
			}

			if !tt.wantModeNull && tr.Mode.ValueString() != tt.wantMode {
				t.Errorf("resource mode = %q, want %q", tr.Mode.ValueString(), tt.wantMode)
			}

			// Data source mapper — same response, separate code path.
			ds := &reportDataSource{}
			dsState := &reportDataSourceModel{}
			if diags := ds.populateState(ctx, dsState, resp); diags.HasError() {
				t.Fatalf("populateState returned errors: %v", diags)
			}

			dsTR := dsState.Config.TimeRange

			if got := dsTR.Unit.IsNull(); got != tt.wantUnitNull {
				t.Errorf("data source unit IsNull() = %v, want %v", got, tt.wantUnitNull)
			}

			if got := dsTR.Mode.IsNull(); got != tt.wantModeNull {
				t.Errorf("data source mode IsNull() = %v, want %v", got, tt.wantModeNull)
			}
		})
	}
}
