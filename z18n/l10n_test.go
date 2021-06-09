package z18n

import (
	"fmt"
	"testing"
)

func TestConvert(t *testing.T) {
	{
		v, k := Convert(int8(123))
		fmt.Printf("%v %v, %v\n", v, v.Type(), k)
		fmt.Println(v.Float())

		v, k = Convert(int16(123))
		fmt.Printf("%v %v, %v\n", v, v.Type(), k)
		fmt.Println(v.Float())
	}
	{
		v, k := Convert([]byte("ASD"))
		fmt.Printf("%v %v, %v\n", v, v.Type(), k)
		fmt.Println(v.String())
	}
	{
		x := "asd"
		v, k := Convert(&x)
		fmt.Printf("%v %v, %v\n", v, v.Type(), k)
		fmt.Println(v.String())
	}

	tests := []struct {
		in interface{}
	}{
		{""},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			_ = tt

			// if have != tt.want {
			// 	t.Errorf("\nhave: %q\nwant: %q", have, tt.want)
			// }
			// if !reflect.DeepEqual(have, tt.want) {
			// 	t.Errorf("\nhave: %#v\nwant: %#v", have, tt.want)
			// }
			// if d := ztest.Diff(have, tt.want); d != "" {
			// 	t.Errorf(d)
			// }
		})
	}
}
