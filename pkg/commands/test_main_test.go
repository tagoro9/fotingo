package commands

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = flag.Set("test.parallel", "1")
	os.Exit(m.Run())
}
