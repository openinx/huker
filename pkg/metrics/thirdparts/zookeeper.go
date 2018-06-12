package thirdparts

type ZookeeperMetricFetcher struct {
	url     string
	host    string
	port    int
	cluster string
}

func (f *ZookeeperMetricFetcher) Pull() (interface{}, error) {
	return nil, nil
}
