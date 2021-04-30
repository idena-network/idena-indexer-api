package service

type ChangeLog interface {
	ForkChangeLog(version string) ([]string, error)
}
