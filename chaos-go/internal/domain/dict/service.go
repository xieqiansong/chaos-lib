package dict

import "strconv"

type DictNode struct {
	ID       int        `json:"ID"`
	Label    string     `json:"Label"`
	Value    string     `json:"Value"`
	ParentID int        `json:"ParentID"`
	Sort     int        `json:"Sort"`
	Status   bool       `json:"Status"`
	Children []DictNode `json:"Children,omitempty"`
}

type DictOption struct {
	ID       int    `json:"ID"`
	Label    string `json:"Label"`
	Value    string `json:"Value"`
	ParentID int    `json:"ParentID"`
}

func SetDictTypeStatus(id int, status bool) (any, error) {
	sid := strconv.Itoa(id)
	if _, err := FindActiveDictTypeByID(sid); err != nil {
		return nil, err
	}
	return SetDictTypeStatusDB(sid, status)
}

func SetDictDataStatus(id int, status bool) (any, error) {
	sid := strconv.Itoa(id)
	if _, err := FindActiveDictDataByID(sid); err != nil {
		return nil, err
	}
	return SetDictDataStatusDB(sid, status)
}

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

		roots = append(roots, *node)
	}
	return roots
}

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
