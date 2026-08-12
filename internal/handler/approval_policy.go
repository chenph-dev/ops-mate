package handler

import configstore "ops-mate/internal/store/config"

// ApprovalPolicyHandler 处理审批策略的前端调用。
type ApprovalPolicyHandler struct {
	policy   *configstore.PolicyStore
	onChange func() // 保存成功后回调（通知 SessionManager 使 Graph 失效）
}

// NewApprovalPolicyHandler 构造 ApprovalPolicyHandler。onChange 可为 nil。
func NewApprovalPolicyHandler(policy *configstore.PolicyStore, onChange func()) *ApprovalPolicyHandler {
	return &ApprovalPolicyHandler{policy: policy, onChange: onChange}
}

func (h *ApprovalPolicyHandler) GetApprovalPolicy() (configstore.ApprovalPolicy, error) {
	return h.policy.GetApprovalPolicy()
}

// SaveApprovalPolicy 保存策略并触发热更新（使已构建 Graph 按新策略重建）。
func (h *ApprovalPolicyHandler) SaveApprovalPolicy(p configstore.ApprovalPolicy) error {
	if err := h.policy.SaveApprovalPolicy(p); err != nil {
		return err
	}
	if h.onChange != nil {
		h.onChange()
	}
	return nil
}
