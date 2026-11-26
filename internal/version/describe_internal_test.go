package version

import (
	"runtime/debug"
	"testing"
)

func TestDescribe(t *testing.T) {
	t.Parallel()

	const rev = "09feb55824caec2060a507b29e573fd64403a73b"
	for name, tc := range map[string]struct {
		settings []debug.BuildSetting
		want     string
	}{
		"no vcs": {[]debug.BuildSetting{{Key: "-buildvcs", Value: "false"}}, "0.3.1"},
		"clean":  {[]debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "false"}}, "0.3.1+09feb55824ca"},
		"dirty":  {[]debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "true"}}, "0.3.1+09feb55824ca.dirty"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := describe("0.3.1", tc.settings); got != tc.want {
				t.Errorf("describe = %q, want %q", got, tc.want)
			}
		})
	}
}
