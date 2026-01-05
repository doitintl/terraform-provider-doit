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

type Budget struct {
	// Alerts List of up to three thresholds defined as percentage of amount
	Alerts []ExternalBudgetAlert `json:"alerts,omitempty"`

	// Amount Budget period amount
	// required: true(if usePrevSpend is false)
	Amount float64 `json:"amount,omitempty"`

	// Collaborators List of permitted users to view/edit the report
	Collaborators []Collaborator `json:"collaborators,omitempty"`

	// Currency Budget currency can be one of: ["USD","ILS","EUR","GBP","AUD","CAD","DKK","NOK","SEK","BRL","SGD","MXN","CHF","MYR","TWD"]
	Currency string `json:"currency"`

	// Description Budget description
	Description string `json:"description,omitempty"`

	// EndPeriod Fixed budget end date
	// required: true(if budget type is fixed)
	EndPeriod int64 `json:"endPeriod,omitempty"`

	// GrowthPerPeriod Periodical growth percentage in recurring budget
	GrowthPerPeriod float64 `json:"growthPerPeriod,omitempty"`

	// Id budget ID, identifying the report
	// in:path
	Id string `json:"id,omitempty"`

	// Metric Budget metric - currently fixed to "cost"
	Metric string `json:"metric,omitempty"`

	// Name Budget Name
	Name string `json:"name"`
	// Public *BudgetPublic `json:"public,omitempty"`
	Public *string `json:"public,omitempty"`

	// Recipients List of emails to notify when reaching alert threshold
	Recipients []string `json:"recipients,omitempty"`

	// RecipientsSlackChannels List of slack channels to notify when reaching alert threshold
	RecipientsSlackChannels []SlackChannel `json:"recipientsSlackChannels,omitempty"`

	// Scope List of attributions that defines that budget scope
	Scope []string `json:"scope"`

	// StartPeriod Budget start Date
	StartPeriod int64 `json:"startPeriod"`

	// TimeInterval Recurring budget interval can be on of: ["day", "week", "month", "quarter","year"]
	TimeInterval string `json:"timeInterval,omitempty"`

	// Type budget type can be one of: ["fixed", "recurring"]
	Type string `json:"type"`

	// UsePrevSpend Use the last period's spend as the target amount for recurring budgets
	UsePrevSpend bool `json:"usePrevSpend,omitempty"`
}

// Budget

// ExternalBudgetAlert defines model for ExternalBudgetAlert.
type ExternalBudgetAlert struct {
	Percentage float64 `json:"percentage,omitempty"`
}

type Collaborator struct {
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
}

// SlackChannel defines model for SlackChannel.
type SlackChannel struct {
	CustomerId string `json:"customerId,omitempty"`
	Id         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Shared     bool   `json:"shared,omitempty"`
	Type       string `json:"type,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
}
