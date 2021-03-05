package sub

import (
	"testing"
)

func TestFail(t *testing.T) {
	t.Errorf("I fail in sub")
}

func TestPass(t *testing.T) {
	// I pass
}
