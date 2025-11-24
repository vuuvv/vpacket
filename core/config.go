package core

import (
	"github.com/vuuvv/errors"
)

type Config struct {
	Protocols      []*Protocol    `yaml:"protocols"`
	DataStructures DataStructures `yaml:"data_structures"` // 保持
}

func (this *Config) Setup() error {
	for _, protocol := range this.Protocols {
		err := protocol.Setup(this.DataStructures)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (this *Config) FindProtocol(token []byte) *Protocol {
	for _, p := range this.Protocols {
		if p.CanParse(token) {
			return p
		}
	}
	return nil
}
