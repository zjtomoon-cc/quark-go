package searches

type DateRange struct {
	Search
}

// 初始化
func (p *DateRange) ParentInit() interface{} {
	p.Component = "dateRangeField"

	return p
}
