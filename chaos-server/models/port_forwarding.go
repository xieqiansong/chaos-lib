package models

type PortForwarding struct {
	Id         int    `gorm:"primaryKey"`
	Name       string ``
	Port       int    ``
	TargetHost string ``
	TargetPort int    ``
	Status     bool   ``
}

func (PortForwarding) TableName() string {
	return "port_forwarding"
}
