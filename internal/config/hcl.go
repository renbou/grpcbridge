package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

type Bridge struct {
	Services []Service `hcl:"service,block"`
}

type Service struct {
	Name   string `hcl:"name,label"`
	Target string `hcl:"target"`
}

func ReadHCL(filename string) (*Bridge, error) {
	cfg := new(Bridge)

	if err := hclsimple.DecodeFile(filename, nil, cfg); err != nil {
		return nil, fmt.Errorf("decoding HCL config file: %w", err)
	}

	return cfg, nil
}
