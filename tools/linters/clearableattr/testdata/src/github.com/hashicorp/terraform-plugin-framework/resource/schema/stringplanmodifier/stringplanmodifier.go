package stringplanmodifier

type PlanModifier struct{}
type UseStateForUnknownResponse struct{}

func UseStateForUnknown() PlanModifier     { return PlanModifier{} }
func RequiresReplace() PlanModifier        { return PlanModifier{} }
