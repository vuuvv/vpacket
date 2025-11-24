package core

type DataStructure struct {
	Fields []*YamlField `yaml:"fields"`
}

type DataStructures map[string]*DataStructure
