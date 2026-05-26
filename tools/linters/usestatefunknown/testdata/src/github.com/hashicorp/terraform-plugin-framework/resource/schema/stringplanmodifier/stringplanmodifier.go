package stringplanmodifier

type PlanModifier struct{}

func UseStateForUnknown() PlanModifier { return PlanModifier{} }
