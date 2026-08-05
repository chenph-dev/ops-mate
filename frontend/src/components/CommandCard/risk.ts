// 前端镜像 internal/einoagent/guardrail/guardrail.go 的 dangerPatterns。
// 两处规则必须保持同步：Go 后端是最终权威判定，本模块仅用于编辑命令时的即时风险提示。
const dangerPatterns: RegExp[] = [
  /rm\s+-[a-zA-Z]*r[a-zA-Z]*f?.*\s\/\s/, // rm -rf ... /
  /rm\s+-[a-zA-Z]*r[a-zA-Z]*f?\s+\/(\*|\s*$)/, // rm -rf / 或 rm -rf /*
  /\bmkfs\b/,
  /\bdd\b.*\bof=\/dev\//,
  /\b(?:shutdown|poweroff|halt|reboot)\b/,
  />\s*\/dev\/(?:sd|nvme|hd|vd)/, // 重定向到块设备
  /:\(\)\s*\{\s*:\|:&\s*\}\s*;/, // fork bomb
];

/** 前端风险判定：命中任一危险模式返回 true（与 Go 端 AssessRisk 的 "high" 等价）。 */
export function isHighRiskCommand(command: string): boolean {
  return dangerPatterns.some((p) => p.test(command.trim()));
}
