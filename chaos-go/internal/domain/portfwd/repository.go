package portfwd

import (
	"errors"

	"chaos-go/internal/framework/config"
	"gorm.io/gorm"
)

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func SshConnExists(id int) (bool, error) {
	db := config.GetDB()
	if db == nil {
		return false, ErrDBUnavailable
	}
	var conn SshConnection
	if err := db.First(&conn, "id = ?", id).Error; err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func FindSshConnByID(id int) (*SshConnection, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var conn SshConnection
	if err := db.First(&conn, "id = ?", id).Error; err != nil {
		if isNotFound(err) {
			return nil, ErrConnNotFound
		}
		return nil, err
	}
	return &conn, nil
}

func CountPortForwardingsByConn(connID int) (int64, error) {
	db := config.GetDB()
	if db == nil {
		return 0, ErrDBUnavailable
	}
	var count int64
	if err := db.Model(&PortForwarding{}).
		Where("ssh_connection_id = ?", connID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func FindPortForwardingByID(id int) (*PortForwarding, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var rule PortForwarding
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		if isNotFound(err) {
			return nil, ErrRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

func FindEnabledPortForwardings() ([]PortForwarding, error) {
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBUnavailable
	}
	var rules []PortForwarding
	if err := db.Where("status = ?", true).
		Order("id ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func SavePortForwarding(rule *PortForwarding) error {
	db := config.GetDB()
	if db == nil {
		return ErrDBUnavailable
	}
	return db.Save(rule).Error
}
