package core

import (
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/vuuvv/errors"
)

type CelEvaluator struct{ prg cel.Program }

func CompileExpression(expr string) (*CelEvaluator, error) {
	env, err := cel.NewEnv(
		cel.Variable("vars", cel.MapType(cel.StringType, cel.DynType)),    // vars为以前的变量
		cel.Variable("fields", cel.MapType(cel.StringType, cel.DynType)),  // fields为当前字段的所有值
		cel.Variable("offsets", cel.MapType(cel.StringType, cel.DynType)), // fields为当前字段的所有值
		cel.Variable("val", cel.DynType),                                  // val为当前字段的值
		cel.Function("now",
			cel.Overload("now_zero_arg", []*cel.Type{}, cel.IntType,
				cel.FunctionBinding(func(_ ...ref.Val) ref.Val {
					// 时间必须在函数调用时生成，不能在 Compile 阶段缓存；否则长生命周期协议会得到过期时间。
					return types.Int(time.Now().UnixMilli())
				}),
			),
		),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &CelEvaluator{prg: prg}, nil
}

//func (e *CelEvaluator) Decode(vars map[string]any, currentVal any) (any, error) {
//	input := map[string]any{"vars": vars}
//	if currentVal != nil {
//		input["val"] = currentVal
//	}
//
//	out, _, err := e.prg.Eval(input)
//	if err != nil {
//		return nil, errors.WithStack(err)
//	}
//	return out.Value(), nil
//}

func (e *CelEvaluator) Execute(ctx *Context) (any, error) {
	input := map[string]any{
		"vars":    ctx.Vars,
		"fields":  ctx.Fields,
		"offsets": ctx.Offsets,
	}
	out, _, err := e.prg.Eval(input)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return out.Value(), nil
}
