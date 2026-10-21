package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	mk := func(kv ...string) *debug.BuildInfo {
		bi := &debug.BuildInfo{}
		for i := 0; i < len(kv); i += 2 {
			bi.Settings = append(bi.Settings, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return bi
	}
	cases := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"no vcs info", mk(), "dev"},
		{"clean", mk("vcs.revision", "3192377abcdef", "vcs.modified", "false"), "dev-3192377"},
		{"dirty", mk("vcs.revision", "3192377abcdef", "vcs.modified", "true"), "dev-3192377-dirty"},
	}
	for _, c := range cases {
		if got := versionFromBuildInfo(c.info); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
