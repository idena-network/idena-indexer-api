package service

type ChangeLog interface {
	ForkChangeLog(version string) (*ChangeLogData, error)
}

type ChangeLogData struct {
	Upgrade uint32
	Changes []string
}
