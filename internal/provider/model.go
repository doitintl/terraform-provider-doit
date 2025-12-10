package provider

// Attribution - .
type Attribution struct {
	Id          string      `json:"id,omitempty"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Formula     string      `json:"formula,omitempty"`
	LastUpdated string      `json:"last_updated,omitempty"`
	Components  []Component `json:"components,omitempty"`
}

// Component - .
type Component struct {
	TypeComponent string   `json:"type"`
	Key           string   `json:"key"`
	Values        []string `json:"values"`
}

// Attribution - .
type AttributionGroup struct {
	Id           string   `json:"id,omitempty"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	LastUpdated  string   `json:"last_updated,omitempty"`
	Attributions []string `json:"attributions"`
}

// Attribution - .
type AttributionGroupGet struct {
	Id           string        `json:"id,omitempty"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	LastUpdated  string        `json:"last_updated"`
	Attributions []Attribution `json:"attributions"`
}

// Report defines model for ExternalReport.
type Report struct {
	// Config Report configuration
	Config ExternalConfig `json:"config,omitempty"`

	// Description Report description
	Description string `json:"description,omitempty"`

	// Id Report id. Leave blank when creating a new report
	Id string `json:"id,omitempty"`

	// Name Report name
	Name string `json:"name"`
}

// ExternalConfig - Report configuration.
type ExternalConfig struct {
	// AdvancedAnalysis Advanced analysis toggles. Each of these can be set independently
	AdvancedAnalysis *AdvancedAnalysis `json:"advancedAnalysis,omitempty"`
	Aggregation      string            `json:"aggregation,omitempty"`
	Currency         string            `json:"currency,omitempty"`
	Dimensions       []Dimension       `json:"dimensions,omitempty"`
	DisplayValues    string            `json:"displayValues,omitempty"`

	// Filters The filters to use in this report
	Filters []ExternalConfigFilter `json:"filters"`

	// Group The groups to use in the report.
	Group []Group `json:"group"`

	// IncludePromotionalCredits Whether to include credits or not.
	// If set, the report must use time interval “month”/”quarter”/”year”
	IncludePromotionalCredits bool            `json:"includePromotionalCredits"`
	Layout                    string          `json:"layout,omitempty"`
	Metric                    *ExternalMetric `json:"metric,omitempty"`

	// MetricFilter {
	// "metric": {
	// "type":  "basic",
	// "value": "cost"
	// },
	// "operator" : "gt",
	// "values" : [50]
	// }
	MetricFilter *ExternalConfigMetricFilter `json:"metricFilter,omitempty"`
	// Splits The splits to use in the report.
	Splits       []ExternalSplit `json:"splits,omitempty"`
	TimeInterval string          `json:"timeInterval,omitempty"`

	// TimeRange Time settings for the report
	// Description: Today is the 17th of April of 2023
	// We set the mode to "last", the amount to 2 and the unit to "day"
	// If includeCurrent is not set, the range will be the 15th and 16th of April
	// If it is, then the range will be 16th and 17th
	TimeRange      *TimeSettings `json:"timeRange,omitempty"`
	SortGroups     string        `json:"sortGroups,omitempty"`
	SortDimensions string        `json:"sortDimensions,omitempty"`
}

// AdvancedAnalysis Advanced analysis toggles. Each of these can be set independently.
type AdvancedAnalysis struct {
	Forecast     bool `json:"forecast"`
	NotTrending  bool `json:"notTrending"`
	TrendingDown bool `json:"trendingDown"`
	TrendingUp   bool `json:"trendingUp"`
}

// Dimension {
// "id" : "sku_description",
// "type" : "fixed"
// }.
type Dimension struct {
	// Id The field to apply to the dimension.
	Id   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
}

// ExternalConfigFilter {
// "id" : "sku_description",
// "type" : "fixed",
// "values" : ["Nearline Storage Iowa", "Nearline Storage Frankfurt"]
// }
//
// When using attributions as a filter both the type and the ID must be "attribution", and the
// values array contains the attribution IDs.
type ExternalConfigFilter struct {
	// Id What field we are filtering on
	Id string `json:"id,omitempty"`

	// Inverse If set, exclude the values
	Inverse bool   `json:"inverse"`
	Type    string `json:"type,omitempty"`

	// Values What values to filter on or exclude
	Values []string `json:"values,omitempty"`
}

// Group defines model for Group.
type Group struct {
	Id    string `json:"id,omitempty"`
	Limit *Limit `json:"limit,omitempty"`
	Type  string `json:"type,omitempty"`
}

// Limit defines model for Limit.
type Limit struct {
	Metric *ExternalMetric `json:"metric,omitempty"`
	Sort   string          `json:"sort,omitempty"`
	// Value The number of items to show
	Value int64 `json:"value,omitempty"`
}

// ExternalMetric defines model for ExternalMetric.
type ExternalMetric struct {
	Type string `json:"type,omitempty"`
	// Value For basic metrics the value can be one of: ["cost", "usage", "savings"]
	//
	// If using custom metrics, the value must refer to an existing custom or calculated metric id.
	Value string `json:"value,omitempty"`
}

type ExternalConfigMetricFilter struct {
	Metric   *ExternalMetric `json:"metric,omitempty"`
	Operator string          `json:"operator,omitempty"`
	Values   []float64       `json:"values,omitempty"`
}

// ExternalSplit - A split to apply to the report.
type ExternalSplit struct {
	// Id ID of the field to split
	Id string `json:"id,omitempty"`

	// IncludeOrigin if set, include the origin
	IncludeOrigin bool            `json:"includeOrigin"`
	Mode          string          `json:"mode,omitempty"`
	Origin        *ExternalOrigin `json:"origin,omitempty"`

	// Targets Targets for the split
	Targets []ExternalSplitTarget `json:"targets,omitempty"`

	// Type Type of the split.
	// The only supported value at the moment: "attribution_group"
	Type string `json:"type,omitempty"`
}

// ExternalOrigin defines model for ExternalOrigin.
type ExternalOrigin struct {
	// Id ID of the origin
	Id string `json:"id,omitempty"`

	// Type Type of the origin.
	// The only supported value at the moment: "attribution"
	Type string `json:"type,omitempty"`
}

// ExternalSplitTarget defines model for ExternalSplitTarget.
type ExternalSplitTarget struct {
	// Id ID of the target
	Id string `json:"id,omitempty"`

	// Type Type of the target.
	// The only supported value at the moment: "target"
	Type string `json:"type,omitempty"`

	// Value Percent of the target, represented in float format. E.g. 30% is 0.3. Must be set only if Split Mode is custom.
	Value float64 `json:"value,omitempty"`
}

// TimeSettings Time settings for the report
// Description: Today is the 17th of April of 2023
// We set the mode to "last", the amount to 2 and the unit to "day"
// If includeCurrent is not set, the range will be the 15th and 16th of April
// If it is, then the range will be 16th and 17th.
type TimeSettings struct {
	Amount         int64  `json:"amount,omitempty"`
	IncludeCurrent bool   `json:"includeCurrent"`
	Mode           string `json:"mode,omitempty"`
	Unit           string `json:"unit,omitempty"`
}
