package observability

type Status struct {
	Ready bool `json:"ready"`
	Model bool `json:"model_loaded"`
}

var ReadyFlag bool
var ModelLoaded bool
