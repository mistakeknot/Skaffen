package subagent

import "testing"

func TestReservationBridge_BuildArgs(t *testing.T) {
	b := &ReservationBridge{icPath: "/usr/bin/ic", projectDir: "/home/mk/projects/test"}

	args := b.buildReserveArgs("sub-1", "*.go", 120)
	expected := []string{
		"coordination", "reserve",
		"--owner=sub-1",
		"--scope=/home/mk/projects/test",
		"--pattern=*.go",
		"--exclusive",
		"--ttl=120",
	}
	if len(args) != len(expected) {
		t.Fatalf("args length = %d, want %d: %v", len(args), len(expected), args)
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestReservationBridge_Unavailable(t *testing.T) {
	b := &ReservationBridge{} // no ic path
	err := b.Reserve("sub-1", []string{"*.go"}, 120)
	if err != nil {
		t.Error("Reserve should succeed (no-op) when ic unavailable")
	}
}

func TestReservationBridge_ReleaseUnavailable(t *testing.T) {
	b := &ReservationBridge{} // no ic path
	b.Release("sub-1")        // should not panic
}
