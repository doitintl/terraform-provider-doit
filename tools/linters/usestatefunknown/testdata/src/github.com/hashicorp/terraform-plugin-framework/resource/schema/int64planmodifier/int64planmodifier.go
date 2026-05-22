package int64planmodifier

type PlanModifier struct{}

func UseStateForUnknown() PlanModifier { return PlanModifier{} }
