package dict

import "strconv"

// DictNode 树节点（用于 tree 动作响应），按 ParentID 嵌套。
type DictNode struct {
	ID       int        `json:"ID"`
	Label    string     `json:"Label"`
	Value    string     `json:"Value"`
	ParentID int        `json:"ParentID"`
	Sort     int        `json:"Sort"`
	Status   bool       `json:"Status"`
	Children []DictNode `json:"Children,omitempty"`
}

// DictOption 选项（用于 options 动作响应，供其他模块下拉使用）。
type DictOption struct {
	ID       int    `json:"ID"`
	Label    string `json:"Label"`
	Value    string `json:"Value"`
	ParentID int    `json:"ParentID"`
}

// SetDictTypeStatus 切换字典类型启用状态：先确认类型存在且未删除，再执行更新。
// 供通用启停路由 crud.RegisterToggle 使用；编排留在用例层，handler 只做配置与转调。
func SetDictTypeStatus(id int, status bool) (any, error) {
	sid := strconv.Itoa(id)
	if _, err := FindActiveDictTypeByID(sid); err != nil {
		return nil, err
	}
	return SetDictTypeStatusDB(sid, status)
}

// SetDictDataStatus 切换字典数据项启用状态：先确认数据项存在且未删除，再执行更新。
func SetDictDataStatus(id int, status bool) (any, error) {
	sid := strconv.Itoa(id)
	if _, err := FindActiveDictDataByID(sid); err != nil {
		return nil, err
	}
	return SetDictDataStatusDB(sid, status)
}

// BuildTree 把某类型下的全部字典数据项构造成树（按 ParentID 嵌套）。
func BuildTree(typeCode string) ([]DictNode, error) {
	rows, err := ListByTypeCode(typeCode)
	if err != nil {
		return nil, err
	}
	return buildTree(rows), nil
}

func buildTree(rows []DictData) []DictNode {
	nodes := make(map[int]*DictNode, len(rows))
	for i := range rows {
		r := rows[i]
		nodes[r.ID] = &DictNode{
			ID:       r.ID,
			Label:    r.Label,
			Value:    r.Value,
			ParentID: r.ParentID,
			Sort:     r.Sort,
			Status:   r.Status,
		}
	}
	roots := make([]DictNode, 0, len(rows))
	for i := range rows {
		r := rows[i]
		node := nodes[r.ID]
		if r.ParentID == 0 {
			roots = append(roots, *node)
			continue
		}
		if parent, ok := nodes[r.ParentID]; ok {
			parent.Children = append(parent.Children, *node)
			continue
		}
		// 父级不存在（被删或脏数据）时退化为根节点，避免丢失。
		roots = append(roots, *node)
	}
	return roots
}

// ListOptions 返回某类型下处于启用状态的数据项（供其他模块下拉）。
func ListOptions(typeCode string) ([]DictOption, error) {
	rows, err := ListEnabledByTypeCode(typeCode)
	if err != nil {
		return nil, err
	}
	opts := make([]DictOption, 0, len(rows))
	for _, r := range rows {
		opts = append(opts, DictOption{
			ID:       r.ID,
			Label:    r.Label,
			Value:    r.Value,
			ParentID: r.ParentID,
		})
	}
	return opts, nil
}
