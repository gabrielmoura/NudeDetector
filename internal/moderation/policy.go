package moderation

type policy struct {
	review float64
	porn   float64
}

func NewPolicy(review, porn float64) Policy {
	return &policy{review: review, porn: porn}
}

func (p *policy) Evaluate(scores ClassificationScores) string {
	if scores.Porn < p.review {
		return "not_pornographic"
	}
	if scores.Porn < p.porn {
		return "review"
	}
	return "pornographic"
}
