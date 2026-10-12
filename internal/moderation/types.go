package moderation

type ClassificationScores struct {
	Drawings float64 `json:"drawings"`
	Hentai   float64 `json:"hentai"`
	Neutral  float64 `json:"neutral"`
	Porn     float64 `json:"porn"`
	Sexy     float64 `json:"sexy"`
}

type ModerationResult struct {
	PornographyScore   float64              `json:"pornography_score"`
	PornographyPercent float64              `json:"pornography_percent"`
	Classification     string               `json:"classification"`
	Scores             ClassificationScores `json:"scores"`
	Thresholds         Thresholds           `json:"thresholds"`
	Model              ModelInfo            `json:"model"`
	Processing         ProcessingInfo       `json:"processing"`
}

type Thresholds struct {
	Review       float64 `json:"review"`
	Pornographic float64 `json:"pornographic"`
}

type ModelInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ProcessingInfo struct {
	Cached bool `json:"cached"`
}

type Classifier interface {
	Classify(logits []float32) (*ClassificationScores, error)
}

type Policy interface {
	Evaluate(scores ClassificationScores) string
}
