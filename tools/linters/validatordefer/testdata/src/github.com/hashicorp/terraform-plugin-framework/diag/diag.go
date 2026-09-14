package diag

type Diagnostics struct{}

func (*Diagnostics) AddAttributeError(_ any, _, _ string) {}

func (*Diagnostics) AddAttributeWarning(_ any, _, _ string) {}

func (*Diagnostics) AddError(_, _ string) {}

func (*Diagnostics) AddWarning(_, _ string) {}
