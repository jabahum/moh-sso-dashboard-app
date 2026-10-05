package report_scheduler

// ModuleResponse describes the initial module without implying that jobs run yet.
type ModuleResponse struct {
 Name string `json:"name"`
 Status string `json:"status"`
 SchedulingEnabled bool `json:"schedulingEnabled"`
}
