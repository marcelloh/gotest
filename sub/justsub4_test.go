package sub

import (
	"testing"
)

func TestFail4(t *testing.T) {
	t.Errorf("I fail in sub")
}

func TestPass4(t *testing.T) {
	// I pass
}
