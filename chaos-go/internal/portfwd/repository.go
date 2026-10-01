package portfwd

import (
	"errors"

	"chaos-go/internal/config"
	"gorm.io/gorm"
)

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// SshConnExists 判断指定 ID 的 SSH 连接是否存在（未软删）。
func SshConnExists(id int) (bool, error) {
	db := config.GetDB()
	if db == nil {
		return false, ErrDBUnavailable
	}
	var conn SshConnection
	if err := db.Where("is_deleted = ?", false).First(&conn, "id = ?", id).Error; err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FindSshConnByID 按 ID 加载未软删的 SSH 连接。
func FindSshConnByID(id string) (*SshConnection, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var conn SshConnection
	if err := db.Where("is_deleted = ?", false).First(&conn, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &conn, nil
}

// CountPortForwardingsByConn 统计引用该 SSH 连接且未软删的转发规则数。
func CountPortForwardingsByConn(connID int) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	var count int64
	if err := db.Model(&PortForwarding{}).
		Where("ssh_connection_id = ? AND is_deleted = ?", connID, false).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// FindPortForwardingByID 按 ID 加载未软删的端口转发规则。
func FindPortForwardingByID(id string) (*PortForwarding, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var rule PortForwarding
	if err := db.Where("is_deleted = ?", false).First(&rule, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// FindEnabledPortForwardings 返回库中所有「期望运行」（status=true 且未软删）的转发规则。
func FindEnabledPortForwardings() ([]PortForwarding, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var rules []PortForwarding
	if err := db.Where("status = ? AND is_deleted = ?", true, false).
		Order("id ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// SavePortForwarding 落库转发规则的变更（含 Status 字段）。
func SavePortForwarding(rule *PortForwarding) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Save(rule).Error
}
