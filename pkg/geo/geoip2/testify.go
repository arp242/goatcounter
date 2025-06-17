package geoip2

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
)

func equal(t *testing.T, want, have any) {
	t.Helper()
	if !reflect.DeepEqual(want, have) {
		t.Errorf("\nhave: %#v\nwant: %#v", have, want)
	}
}

func notZero(t *testing.T, have any) {
	t.Helper()
	want := reflect.New(reflect.TypeOf(have)).Elem().Interface()
	if reflect.DeepEqual(want, have) {
		t.Errorf("\nhave: %#v\nwant non-zero value", have)
	}
}

func isFalse(t *testing.T, have bool) {
	t.Helper()
	if have {
		t.Errorf("\nhave: %v\nwant: false", have)
	}
}

func isTrue(t *testing.T, have bool) {
	t.Helper()
	if !have {
		t.Errorf("\nhave: %v\nwant: true", have)
	}
}

func inEpsilon(t *testing.T, want, have any, epsilon float64) {
	t.Helper()
	if math.IsNaN(epsilon) {
		t.Error("epsilon must not be NaN")
		return
	}
	actualEpsilon, err := calcRelativeError(want, have)
	if err != nil {
		t.Error(err)
		return
	}
	if math.IsNaN(actualEpsilon) {
		t.Error("relative error is NaN")
		return
	}
	if actualEpsilon > epsilon {
		t.Errorf("relative error too high\nwant: %#v\nhave: %#v", epsilon, actualEpsilon)
	}
}
func calcRelativeError(expected, actual interface{}) (float64, error) {
	af, aok := toFloat(expected)
	bf, bok := toFloat(actual)
	if !aok || !bok {
		return 0, fmt.Errorf("parameters must be numerical")
	}
	if math.IsNaN(af) && math.IsNaN(bf) {
		return 0, nil
	}
	if math.IsNaN(af) {
		return 0, errors.New("expected value must not be NaN")
	}
	if af == 0 {
		return 0, fmt.Errorf("expected value must have a value other than zero to calculate the relative error")
	}
	if math.IsNaN(bf) {
		return 0, errors.New("actual value must not be NaN")
	}

	return math.Abs(af-bf) / math.Abs(af), nil
}
func toFloat(x interface{}) (float64, bool) {
	var xf float64
	xok := true

	switch xn := x.(type) {
	case uint:
		xf = float64(xn)
	case uint8:
		xf = float64(xn)
	case uint16:
		xf = float64(xn)
	case uint32:
		xf = float64(xn)
	case uint64:
		xf = float64(xn)
	case int:
		xf = float64(xn)
	case int8:
		xf = float64(xn)
	case int16:
		xf = float64(xn)
	case int32:
		xf = float64(xn)
	case int64:
		xf = float64(xn)
	case float32:
		xf = float64(xn)
	case float64:
		xf = xn
	case time.Duration:
		xf = float64(xn)
	default:
		xok = false
	}

	return xf, xok
}
