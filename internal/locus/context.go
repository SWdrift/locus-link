package locus

import "os"

func LoadActiveRegistry(root string) (*Registry, error) {
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root, err = DiscoverRegistry(cwd)
		if err != nil {
			return nil, err
		}
	}
	return LoadRegistry(root)
}

func ObservationVantage(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return "host:" + host, nil
}
