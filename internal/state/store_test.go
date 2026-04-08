package state

import "testing"

func TestStoreSetAndSubscribe(t *testing.T) {
	store := NewStore(AppState{Model: "m1", PermissionMode: "default", Theme: "default"})
	calls := 0
	unsub := store.Subscribe(func(s AppState) {
		calls++
		if s.Model != "m2" {
			t.Fatalf("unexpected model in listener: %s", s.Model)
		}
	})
	store.Set(AppState{Model: "m2", PermissionMode: "default", Theme: "default"})
	if calls != 1 {
		t.Fatalf("expected listener to be called once, got %d", calls)
	}
	unsub()
	store.Set(AppState{Model: "m3", PermissionMode: "default", Theme: "default"})
	if calls != 1 {
		t.Fatalf("expected unsubscribe to stop notifications")
	}
}
