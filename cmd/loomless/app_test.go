package main

import "testing"

func TestLaunchedAsAppNeverHijacksCLIArgs(t *testing.T) {
	for _, args := range [][]string{{"serve"}, {"record", "start", "x"}, {"-psn_0_1", "serve"}} {
		if launchedAsApp(args) {
			t.Errorf("launchedAsApp(%q) = true; a typed command must run as typed", args)
		}
	}
}
