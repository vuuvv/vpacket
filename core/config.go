package core

import (
	"bytes"
	"github.com/vuuvv/errors"
	"gopkg.in/yaml.v3"
	"os"
)

type Scheme struct {
	Protocols      []*Protocol    `yaml:"protocols"`
	DataStructures DataStructures `yaml:"data_structures"` // 保持
}

func NewScheme(content []byte) (*Scheme, error) {
	scheme := &Scheme{}
	err := yaml.NewDecoder(bytes.NewReader(content)).Decode(scheme)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return scheme, nil
}

func NewSchemeFromFile(configFile string) (*Scheme, error) {
	f, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	scheme := &Scheme{}
	err = yaml.NewDecoder(f).Decode(scheme)
	if err != nil {
		return nil, err
	}
	return scheme, nil
}

func (this *Scheme) Setup() error {
	for _, protocol := range this.Protocols {
		err := protocol.Setup(this.DataStructures)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (this *Scheme) FindProtocol(token []byte) *Protocol {
	for _, p := range this.Protocols {
		if p.CanParse(token) {
			return p
		}
	}
	return nil
}
