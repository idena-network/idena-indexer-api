package indexer

type NetworkSizeLoader struct {
	api Api
}

func NewNetworkSizeLoader(api Api) *NetworkSizeLoader {
	res := &NetworkSizeLoader{
		api: api,
	}
	return res
}

func (l *NetworkSizeLoader) Load() (uint64, error) {
	return l.api.OnlineIdentitiesCount()
}
