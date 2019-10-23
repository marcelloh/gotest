package main

import (
	"testing"
)

func TestFail(t *testing.T) {
	t.Errorf("I fail")
}

func TestPass(t *testing.T) {
	// I pass
}
