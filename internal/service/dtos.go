package service

type PeriodScore struct {
	Period string
	Score  float64
}

type AggregatedCategoryScores struct {
	CategoryName         string
	TotalRatings         int
	OverallCategoryScore float64
	PeriodScores         []PeriodScore
}

type TicketScores struct {
	TicketID       int64
	CategoryScores map[string]float64
}

type PeriodChange struct {
	CurrentPeriodScore  float64
	PreviousPeriodScore float64
	ChangePercentage    float64
}
