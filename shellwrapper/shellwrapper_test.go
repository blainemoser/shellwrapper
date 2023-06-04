package shellwrapper

import "testing"

func TestNew(t *testing.T) {
	sh := NewShell()
	sh.Display("I am a fish")
}
