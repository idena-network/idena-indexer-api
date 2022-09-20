package service

type ChangeLog interface {
	ForkChangeLog(version string) (*ChangeLogData, error)
	Url(upgrade uint32) string
}

type ChangeLogData struct {
	Upgrade uint32
	Url     string
	Changes []string
}
