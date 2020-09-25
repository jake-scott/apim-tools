package cmd

import "testing"

func sliceEq(a, b []interface{}) bool {

	// If one is nil, the other must also be nil.
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestSliceSubtract(t *testing.T) {
	tests := []struct {
		a    []interface{}
		b    []interface{}
		want []interface{}
	}{
		{
			[]interface{}{"one", "two", "three", "four", "five"},
			[]interface{}{"three", "four", "ten"},
			[]interface{}{"one", "two", "five"},
		},
		{
			[]interface{}{"one", "two", "three", "four", "five"},
			[]interface{}{"ten"},
			[]interface{}{"one", "two", "three", "four", "five"},
		},
		{
			[]interface{}{"one", "two", "three", "four", "five"},
			[]interface{}{},
			[]interface{}{"one", "two", "three", "four", "five"},
		},
		{
			[]interface{}{},
			[]interface{}{"xyz"},
			[]interface{}{},
		},
		{
			[]interface{}{},
			[]interface{}{},
			[]interface{}{},
		},
		{
			[]interface{}{"one", "two", "three", "four", "five"},
			[]interface{}{10},
			[]interface{}{"one", "two", "three", "four", "five"},
		},
		{
			[]interface{}{102, 200, 824, 402},
			[]interface{}{200},
			[]interface{}{102, 824, 402},
		},
		{
			[]interface{}{102, 200, 824, 402, "foo"},
			[]interface{}{200},
			[]interface{}{102, 824, 402, "foo"},
		},
	}

	for _, tt := range tests {
		c := sliceSubtract(tt.a, tt.b)

		if !sliceEq(c, tt.want) {
			t.Errorf("Got %+v, wanted %+v", c, tt.want)
		}
	}

}
