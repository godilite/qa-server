package models

type TicketQualityEvaluation struct {
	Value  int
	Weight float64
}

type TicketCategoryScore struct {
	TicketID int64
	Category string
	Score    float64
}

type AggregatedCategoryData struct {
	Category                string
	Period                  string
	TotalWeightedEvaluation float64
	TotalWeight             float64
	EvaluationCount         int
}

type OverallRatingResult struct {
	Score float64
	Count int64
}
