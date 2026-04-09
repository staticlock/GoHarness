package vim

import (
	"testing"
)

func TestToggleVimMode(t *testing.T) {
	if ToggleVimMode(true) != false {
		t.Fatalf("toggle true should return false")
	}
	if ToggleVimMode(false) != true {
		t.Fatalf("toggle false should return true")
	}
}
