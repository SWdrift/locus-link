package locus

import (
	"errors"
	"os"
)

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

func RequiredRuntime(registry *Registry, from, vantage string) (RuntimeContext, error) {
	resolvedVantage, err := ObservationVantage(vantage)
	if err != nil {
		return RuntimeContext{}, err
	}
	if from == "" {
		return RuntimeContext{}, errors.New("--from is required for this command")
	}
	currentEntity, err := registry.ResolveEntity(from)
	if err != nil {
		return RuntimeContext{}, err
	}
	return RuntimeContext{CurrentEntity: currentEntity, Vantage: resolvedVantage}, nil
}
