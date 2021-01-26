package sub

import (
	"testing"
)

func TestFail3(t *testing.T) {
	t.Errorf("I fail in sub")
}

func TestPass3(t *testing.T) {
	// I pass
}
