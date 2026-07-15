package resource

import "testing"

type TestCase struct{}

func Test(t *testing.T, c TestCase)         {}
func ParallelTest(t *testing.T, c TestCase) {}
