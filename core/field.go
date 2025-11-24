package core

type YamlSwitchCase struct {
	Value  any          `yaml:"value"`
	Ref    string       `yaml:"ref"`    // 外部引用
	Fields []*YamlField `yaml:"fields"` // 内联定义
}

type YamlField struct {
	Name      string       `yaml:"name"`
	Bits      int          `yaml:"bits"`
	Type      string       `yaml:"type"`
	Size      int          `yaml:"size"`
	SizeExpr  string       `yaml:"size_expr"`
	Endian    string       `yaml:"endian"` // 字节序, big: 大端, little: 小端, 默认大端
	Check     string       `yaml:"check"`
	Condition string       `yaml:"condition"`
	Formula   string       `yaml:"formula"`
	Then      []*YamlField `yaml:"then"`

	// Switch 相关的字段
	Field         string            `yaml:"field"`
	Cases         []*YamlSwitchCase `yaml:"cases"`
	DefaultRef    string            `yaml:"default_ref"`    // 默认外部引用
	DefaultFields []*YamlField      `yaml:"default_fields"` // 默认内联定义

	// crc
	Crc      string `yaml:"crc"`       // 标记为 CRC 字段
	CrcStart string `yaml:"crc_start"` // 起始偏移 CEL 表达式
	CrcEnd   string `yaml:"crc_end"`

	// 声明是否回填
	EncodeAfter string `yaml:"encode_after"` // 在哪个字段编码完成后进行回填
}
