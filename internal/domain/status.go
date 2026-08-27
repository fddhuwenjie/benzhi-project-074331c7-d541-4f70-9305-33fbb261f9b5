package domain

type ProjectStatus string

const (
	StatusDraft       ProjectStatus = "草拟"
	StatusFrozen      ProjectStatus = "已冻结规格"
	StatusChecking    ProjectStatus = "检查中"
	StatusRemediation ProjectStatus = "待整改"
	StatusApproval    ProjectStatus = "待批准"
	StatusPublished   ProjectStatus = "已发布"
)

func CanTransition(from, to ProjectStatus) bool {
	allowed := map[ProjectStatus]map[ProjectStatus]bool{
		StatusDraft:       {StatusFrozen: true},
		StatusFrozen:      {StatusChecking: true},
		StatusChecking:    {StatusRemediation: true, StatusApproval: true},
		StatusRemediation: {StatusChecking: true, StatusApproval: true},
		StatusApproval:    {StatusChecking: true, StatusPublished: true},
	}
	return allowed[from][to]
}
