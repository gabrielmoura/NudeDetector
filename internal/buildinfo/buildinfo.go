package buildinfo

import "fmt"

const ONNXRuntimeVersion = "1.22.0"

// These values are replaced by the release build with -ldflags -X.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func Current() Info {
	return Info{Version: Version, Commit: Commit, BuildTime: BuildTime}
}

func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildTime)
}
