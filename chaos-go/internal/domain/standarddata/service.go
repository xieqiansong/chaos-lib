package standarddata

import "strconv"

func ToggleStatus(id int, status bool) (any, error) {
	sid := strconv.Itoa(id)
	if _, err := FindActiveByID(sid); err != nil {
		return nil, err
	}
	return SetStatus(sid, status)
}
