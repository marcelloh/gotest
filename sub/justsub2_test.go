package sub

import (
	"testing"
)

func TestFail2(t *testing.T) {
	t.Errorf("I fail in sub")
}

func TestPass2(t *testing.T) {
	// I passed true
}
