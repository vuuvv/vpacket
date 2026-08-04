package core

import (
	"testing"
	"time"
)

func TestCelEvaluatorNowIsUnixMilli(t *testing.T) {
	evaluator, err := CompileExpression("now()")
	if err != nil {
		t.Fatalf("编译 now() 表达式失败: %v", err)
	}

	before := time.Now().UnixMilli()
	value, err := evaluator.Execute(NewContext(nil))
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("执行 now() 表达式失败: %v", err)
	}

	now, ok := value.(int64)
	if !ok {
		t.Fatalf("now() 返回类型错误，期望 int64，实际 %T", value)
	}
	if now < before || now > after {
		t.Fatalf("now() 不是当前毫秒时间戳: now=%d, 范围=[%d, %d]", now, before, after)
	}
}

func TestCelEvaluatorNowMustBeCalled(t *testing.T) {
	_, err := CompileExpression("now")
	if err == nil {
		t.Fatal("now 只能作为函数调用，不应被识别为变量")
	}
}
