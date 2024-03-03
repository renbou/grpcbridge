package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/renbou/grpcbridge"
)

type config struct {
	Services []serviceConfig `hcl:"service,block"`
}

type serviceConfig struct {
	Name   string `hcl:"name,label"`
	Target string `hcl:"target"`
}

func ReadHCL(filename string) (*grpcbridge.Config, error) {
	var rawCfg config

	if err := hclsimple.DecodeFile(filename, nil, &rawCfg); err != nil {
		return nil, fmt.Errorf("decoding HCL config file: %w", err)
	}

	cfg := &grpcbridge.Config{
		Services: make(map[string]grpcbridge.ServiceConfig, len(rawCfg.Services)),
	}

	for i := range rawCfg.Services {
		svcCfg := &rawCfg.Services[i]
		cfg.Services[svcCfg.Name] = grpcbridge.ServiceConfig{
			Name:   svcCfg.Name,
			Target: svcCfg.Target,
		}
	}

	return cfg, nil
}
