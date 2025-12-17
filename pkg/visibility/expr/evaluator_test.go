package expr

import (
	"testing"

	"github.com/goliatone/go-formgen/pkg/visibility"
)

func TestEvaluatorBooleanComparison(t *testing.T) {
	t.Parallel()

	eval := New()

	ok, err := eval.Eval("threshold", "enabled == true", visibility.Context{
		Values: map[string]any{"enabled": true},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true")
	}

	ok, err = eval.Eval("threshold", "enabled == true", visibility.Context{
		Values: map[string]any{"enabled": "true"},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for string true")
	}
}

func TestEvaluatorTruthyAndNot(t *testing.T) {
	t.Parallel()

	eval := New()

	ok, err := eval.Eval("threshold", "enabled", visibility.Context{
		Values: map[string]any{"enabled": true},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true")
	}

	ok, err = eval.Eval("threshold", "!enabled", visibility.Context{
		Values: map[string]any{"enabled": false},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for !false")
	}
}

func TestEvaluatorDotLookup(t *testing.T) {
	t.Parallel()

	eval := New()

	ok, err := eval.Eval("cta.headline", `cta.headline != ""`, visibility.Context{
		Values: map[string]any{"cta.headline": "Hello"},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for flattened dotted key")
	}

	ok, err = eval.Eval("cta.headline", `cta.headline == "Hello"`, visibility.Context{
		Values: map[string]any{
			"cta": map[string]any{
				"headline": "Hello",
			},
		},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for nested map lookup")
	}
}

func TestEvaluatorNullLiteral(t *testing.T) {
	t.Parallel()

	eval := New()

	ok, err := eval.Eval("threshold", "missing == null", visibility.Context{
		Values: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for missing == null")
	}

	ok, err = eval.Eval("threshold", "enabled != null", visibility.Context{
		Values: map[string]any{"enabled": false},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for present != null")
	}
}

func TestEvaluatorBooleanComposition(t *testing.T) {
	t.Parallel()

	eval := New()

	ok, err := eval.Eval("threshold", `enabled == true && role == "admin"`, visibility.Context{
		Values: map[string]any{
			"enabled": true,
			"role":    "admin",
		},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for conjunction")
	}

	ok, err = eval.Eval("threshold", `enabled == true && role == "admin"`, visibility.Context{
		Values: map[string]any{
			"enabled": true,
			"role":    "user",
		},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected false for conjunction mismatch")
	}

	ok, err = eval.Eval("threshold", `enabled == true || role == "admin"`, visibility.Context{
		Values: map[string]any{
			"enabled": false,
			"role":    "admin",
		},
	})
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true for disjunction")
	}
}

