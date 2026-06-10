package booldefault

type BoolDefault struct{}

func StaticBool(v bool) BoolDefault { return BoolDefault{} }
