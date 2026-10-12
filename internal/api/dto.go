package api

import (
	"github.com/gabrielmoura/NudeDetector/internal/detector"
	"github.com/gabrielmoura/NudeDetector/internal/moderation"
)

type URLRequest struct {
	URL     string `json:"url"`
	Details bool   `json:"details,omitempty"`
}

// ModerationResponse keeps the public response compact by returning class
// probabilities and NudeNet detections only when details is enabled in the
// request body.
type ModerationResponse struct {
	PornographyScore   float64                          `json:"pornography_score"`
	PornographyPercent float64                          `json:"pornography_percent"`
	Classification     string                           `json:"classification"`
	Scores             *moderation.ClassificationScores `json:"scores,omitempty"`
	Details            []detector.Detection             `json:"details,omitempty"`
	Thresholds         moderation.Thresholds            `json:"thresholds"`
	Model              moderation.ModelInfo             `json:"model"`
	Processing         moderation.ProcessingInfo        `json:"processing"`
}

func newModerationResponse(
	result *moderation.ModerationResult,
	details bool,
	detections []detector.Detection,
) ModerationResponse {
	response := ModerationResponse{
		PornographyScore:   result.PornographyScore,
		PornographyPercent: result.PornographyPercent,
		Classification:     result.Classification,
		Thresholds:         result.Thresholds,
		Model:              result.Model,
		Processing:         result.Processing,
	}
	if details {
		scores := result.Scores
		response.Scores = &scores
		response.Details = detections
	}
	return response
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
