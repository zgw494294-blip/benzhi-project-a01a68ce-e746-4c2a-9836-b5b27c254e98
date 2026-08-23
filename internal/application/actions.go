package application

import (
	"sort"

	"sensory-blind-review/internal/domain"
)

type ActionDescriptor struct {
	Key     domain.Permission `json:"key"`
	Label   string            `json:"label"`
	Allowed bool              `json:"allowed"`
	Reason  string            `json:"reason,omitempty"`
	Method  string            `json:"method,omitempty"`
	Path    string            `json:"path,omitempty"`
}

var actionLabels = map[domain.Permission]string{
	domain.PermissionConfigure:     "修改会话配置",
	domain.PermissionManageSamples: "维护样品",
	domain.PermissionFreeze:        "冻结盲码计划",
	domain.PermissionStart:         "启动评分采集",
	domain.PermissionSubmit:        "提交盲样评分",
	domain.PermissionClose:         "关闭评分采集",
	domain.PermissionResolve:       "裁定核验发现",
	domain.PermissionApprove:       "批准解盲",
	domain.PermissionSeal:          "封存归档",
	domain.PermissionAudit:         "查看审计轨迹",
}

func (a *Service) Actions(id, actor string) ([]ActionDescriptor, error) {
	session, err := a.Store.GetSession(id)
	if err != nil {
		return nil, err
	}
	authorizations := session.AvailablePermissions(actor)
	actions := make([]ActionDescriptor, 0, len(authorizations))
	for permission, authorization := range authorizations {
		descriptor := ActionDescriptor{Key: permission, Label: actionLabels[permission], Allowed: authorization.Allowed, Reason: authorization.Reason}
		switch permission {
		case domain.PermissionConfigure:
			descriptor.Method, descriptor.Path = "PATCH", "/api/sessions/"+id
		case domain.PermissionManageSamples:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/samples"
		case domain.PermissionFreeze:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/freeze"
		case domain.PermissionStart:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/start"
		case domain.PermissionSubmit:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/evaluations"
		case domain.PermissionClose:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/close"
		case domain.PermissionResolve:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/findings/{findingID}/resolve"
		case domain.PermissionApprove:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/reveal/approve"
		case domain.PermissionSeal:
			descriptor.Method, descriptor.Path = "POST", "/api/sessions/"+id+"/seal"
		case domain.PermissionAudit:
			descriptor.Method, descriptor.Path = "GET", "/api/sessions/"+id+"/audit"
		}
		actions = append(actions, descriptor)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].Key < actions[j].Key })
	return actions, nil
}
