package core

import "testing"

func TestSelectTurnURLRoundRobinAndFallback(t *testing.T) {
	urls := []string{"a:1", "b:1", "c:1"}
	noneBanned := func(string) bool { return false }
	for worker, want := range []string{"a:1", "b:1", "c:1", "a:1"} {
		got, ok := selectTurnURL(urls, worker+1, 0, noneBanned)
		if !ok || got != want {
			t.Fatalf("worker %d: got %q,%v want %q,true", worker+1, got, ok, want)
		}
	}

	got, ok := selectTurnURL(urls, 1, 0, func(s string) bool { return s == "a:1" })
	if !ok || got != "b:1" {
		t.Fatalf("fallback: got %q,%v want b:1,true", got, ok)
	}
	got, _ = selectTurnURL(urls, 1, 1, noneBanned)
	if got != "b:1" {
		t.Fatalf("retry rotation: got %q want b:1", got)
	}
}

func TestConfigRacersAreBounded(t *testing.T) {
	for i := 0; i < 9; i++ {
		want := i < configRacerLimit
		if got := shouldRaceForConfig(true, i, 0); got != want {
			t.Errorf("worker %d races = %v, want %v", i, got, want)
		}
	}
	if shouldRaceForConfig(true, 0, 1) {
		t.Error("config racers continue after config delivery")
	}
	if shouldRaceForConfig(false, 0, 0) {
		t.Error("config racers started when config is disabled")
	}
}

func TestNormalizeWorkers(t *testing.T) {
	cases := map[int]int{-1: 9, 4: 9, 9: 9, 16: 9, 18: 18, 999: 108}
	for in, want := range cases {
		if got := normalizeWorkers(in); got != want {
			t.Errorf("normalizeWorkers(%d)=%d, want %d", in, got, want)
		}
	}
}
