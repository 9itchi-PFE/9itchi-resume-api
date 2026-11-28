package api

// UsageItem is one limit of the plan and how much of it is used. Limit is nil when the plan sets none.
type UsageItem struct {
	Limit *int `json:"limit"`
	Used  int  `json:"used"`
}

// UserUsageResponse is what the team sees of a student's use of what the plan allows in resume-api: counted the
// way the limit is. Resumes counts over the life of the account, Downloads over the current period of the
// subscription (ever without one), AIRewrites over the calendar month (UTC).
type UserUsageResponse struct {
	Resumes    UsageItem `json:"resumes"`
	Downloads  UsageItem `json:"downloads"`
	AIRewrites UsageItem `json:"ai_rewrites"`
}
