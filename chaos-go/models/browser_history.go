package models

type BrowserHistory struct {
	ID            string  `gorm:"primaryKey"`
	LastVisitTime float64 ``
	Title         string  ``
	TypeCount     int     ``
	Url           string  ``
	VisitCount    int     ``
}

type BrowserHistoryVisit struct {
	ID               string  `gorm:"primaryKey"`
	HistoryID        string  ``
	IsLocal          bool    ``
	ReferringVisitID string  ``
	Transition       string  ``
	VisitID          string  ``
	VisitTime        float64 ``
}
