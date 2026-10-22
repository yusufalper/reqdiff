// Package report defines the versioned CLI output and its renderers.
package report

type Difference struct {
	Scope      string `json:"scope"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	LeftType   string `json:"left_type,omitempty"`
	RightType  string `json:"right_type,omitempty"`
	LeftValue  *any   `json:"left_value,omitempty"`
	RightValue *any   `json:"right_value,omitempty"`
}
type Endpoint struct {
	URL        string `json:"url"`
	Status     *int   `json:"status"`
	DurationNS int64  `json:"duration_ns"`
	BodyBytes  int    `json:"body_bytes"`
	BodyKind   string `json:"body_kind"`
}
type Latency struct {
	LeftNS      int64  `json:"left_ns"`
	RightNS     int64  `json:"right_ns"`
	DeltaNS     int64  `json:"delta_ns"`
	ThresholdNS *int64 `json:"threshold_ns,omitempty"`
	Exceeded    bool   `json:"exceeded"`
}
type Error struct {
	Side    string `json:"side"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e Error) Error() string { return e.Message }

type Warning struct {
	Side    string `json:"side"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Report struct {
	SchemaVersion int          `json:"schema_version"`
	Mode          string       `json:"mode"`
	Result        string       `json:"result"`
	Left          Endpoint     `json:"left"`
	Right         Endpoint     `json:"right"`
	Differences   []Difference `json:"differences"`
	Latency       Latency      `json:"latency"`
	Warnings      []Warning    `json:"warnings"`
	Errors        []Error      `json:"errors"`
}

func New(mode string) Report {
	return Report{SchemaVersion: 1, Mode: mode, Result: "equal", Differences: []Difference{}, Warnings: []Warning{}, Errors: []Error{}}
}
