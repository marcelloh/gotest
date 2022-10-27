package main_test

import (
	"testing"
)

func TestFail(t *testing.T) {
	t.Errorf("I fail")
}

func TestPass(t *testing.T) {
	// I pass
}

func TestPass2(t *testing.T) {
	demo = "" //nolint:typecheck // this is a test for a non declared variable
}
