package tarefresh

import (
	"testing"
)

func TestIsCheckRefreshable(t *testing.T) {
	refreshable := isCheckRefreshable("I'm Refreshable")
	if !refreshable {
		t.Fatalf("isCheckRefreshable() is false, \n want true")
	}
	notRefreshable := isCheckRefreshable("AWS Direct Connect Connection Redundancy")
	if notRefreshable {
		t.Fatalf("isCheckRefreshable() is true, \n want false")
	}

}
