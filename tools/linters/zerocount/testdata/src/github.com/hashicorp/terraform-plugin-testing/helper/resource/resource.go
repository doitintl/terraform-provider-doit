package resource

type TestCheckFunc func() error

func TestCheckResourceAttr(name, key, value string) TestCheckFunc { return nil }

func TestCheckResourceAttrSet(name, key string) TestCheckFunc { return nil }
