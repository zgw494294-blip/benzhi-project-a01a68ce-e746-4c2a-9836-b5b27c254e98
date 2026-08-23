package domain

type Role string

const (
	RoleHost        Role = "host"
	RoleReviewer    Role = "reviewer"
	RoleQuality     Role = "quality"
	RoleIndependent Role = "independent"
	RoleObserver    Role = "observer"
)

type Permission string

const (
	PermissionConfigure     Permission = "configure"
	PermissionManageSamples Permission = "manage_samples"
	PermissionFreeze        Permission = "freeze"
	PermissionStart         Permission = "start"
	PermissionSubmit        Permission = "submit"
	PermissionClose         Permission = "close"
	PermissionResolve       Permission = "resolve"
	PermissionApprove       Permission = "approve"
	PermissionSeal          Permission = "seal"
	PermissionAudit         Permission = "audit"
)

type Authorization struct {
	Allowed bool          `json:"allowed"`
	Role    Role          `json:"role"`
	Reason  string        `json:"reason,omitempty"`
	Status  SessionStatus `json:"status"`
}

func (s *TastingSession) RoleOf(userID string) Role {
	if userID == s.HostUserID {
		return RoleHost
	}
	if s.IsReviewer(userID) {
		return RoleReviewer
	}
	if userID == "" {
		return RoleObserver
	}
	return RoleQuality
}

func (s *TastingSession) Authorize(userID string, permission Permission) Authorization {
	role := s.RoleOf(userID)
	result := Authorization{Role: role, Status: s.Status}
	if s.Status == StatusSealed {
		result.Reason = "会话已封存，只允许查询"
		return result
	}
	switch permission {
	case PermissionConfigure, PermissionManageSamples:
		result.Allowed = role == RoleHost && s.Status == StatusDraft
		result.Reason = "仅主持人可在草稿状态修改配置"
	case PermissionFreeze:
		result.Allowed = role == RoleHost && s.Status == StatusDraft
		result.Reason = "仅主持人可冻结草稿会话"
	case PermissionStart:
		result.Allowed = role == RoleHost && s.Status == StatusFrozen
		result.Reason = "仅主持人可启动已冻结会话"
	case PermissionSubmit:
		result.Allowed = role == RoleReviewer && (s.Status == StatusCollecting || s.Status == StatusVerifying)
		result.Reason = "仅本会话评审员可在采集或重评期间提交"
	case PermissionClose:
		result.Allowed = role == RoleHost && s.Status == StatusCollecting
		result.Reason = "仅主持人可关闭采集"
	case PermissionResolve:
		result.Allowed = role == RoleQuality && s.Status == StatusVerifying
		result.Reason = "仅独立质量复核员可裁定发现"
	case PermissionApprove:
		result.Allowed = role == RoleQuality && s.Status == StatusVerifying && s.AllFindingsResolved()
		result.Reason = "仅独立人员可在发现全部解决后批准解盲"
	case PermissionSeal:
		result.Allowed = role == RoleQuality && s.Status == StatusRevealed
		result.Reason = "仅独立质量复核员可封存已解盲会话"
	case PermissionAudit:
		result.Allowed = role != RoleReviewer && userID != ""
		result.Reason = "评审员不能查看完整审计信息"
	default:
		result.Reason = "未知权限"
	}
	if result.Allowed {
		result.Reason = ""
	}
	return result
}

func (s *TastingSession) AvailablePermissions(userID string) map[Permission]Authorization {
	permissions := []Permission{PermissionConfigure, PermissionManageSamples, PermissionFreeze, PermissionStart, PermissionSubmit, PermissionClose, PermissionResolve, PermissionApprove, PermissionSeal, PermissionAudit}
	out := make(map[Permission]Authorization, len(permissions))
	for _, permission := range permissions {
		out[permission] = s.Authorize(userID, permission)
	}
	return out
}
