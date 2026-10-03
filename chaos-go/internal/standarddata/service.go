package standarddata

import "strconv"

// ToggleStatus 按 ID 切换启用状态：先确认记录存在且未删除，再执行更新。
// 供通用启停路由 crud.RegisterToggle 使用；编排留在用例层，handler 只做配置与转调。
func ToggleStatus(id int, status bool) (any, error) {
	sid := strconv.Itoa(id)
	if _, err := FindActiveByID(sid); err != nil {
		return nil, err
	}
	return SetStatus(sid, status)
}
