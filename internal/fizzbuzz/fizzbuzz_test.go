package fizzbuzz_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/20101997/fizz_buzz/internal/fizzbuzz"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name   string
		params fizzbuzz.Params
		want   string
	}{
		{
			name:   "classic fizz-buzz",
			params: fizzbuzz.Params{Int1: 3, Int2: 5, Limit: 16, Str1: "fizz", Str2: "buzz"},
			want:   "1,2,fizz,4,buzz,fizz,7,8,fizz,buzz,11,fizz,13,14,fizzbuzz,16",
		},
		{
			name:   "custom words and divisors",
			params: fizzbuzz.Params{Int1: 2, Int2: 3, Limit: 6, Str1: "a", Str2: "b"},
			want:   "1,a,b,a,5,ab",
		},
		{
			name:   "same divisors always yield both words",
			params: fizzbuzz.Params{Int1: 2, Int2: 2, Limit: 4, Str1: "a", Str2: "b"},
			want:   "1,ab,3,ab",
		},
		{
			name:   "one divisor is a multiple of the other",
			params: fizzbuzz.Params{Int1: 2, Int2: 4, Limit: 8, Str1: "a", Str2: "b"},
			want:   "1,a,3,ab,5,a,7,ab",
		},
		{
			name:   "divisor of 1 replaces every number",
			params: fizzbuzz.Params{Int1: 1, Int2: 2, Limit: 3, Str1: "a", Str2: "b"},
			want:   "a,ab,a",
		},
		{
			name:   "divisors above the limit never match",
			params: fizzbuzz.Params{Int1: 10, Int2: 20, Limit: 3, Str1: "a", Str2: "b"},
			want:   "1,2,3",
		},
		{
			name:   "empty words",
			params: fizzbuzz.Params{Int1: 2, Int2: 3, Limit: 3},
			want:   "1,,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fizzbuzz.Generate(tt.params)
			if err != nil {
				t.Fatalf("Generate() unexpected error: %v", err)
			}
			if want := strings.Split(tt.want, ","); !slices.Equal(got, want) {
				t.Errorf("Generate() = %v, want %v", got, want)
			}
		})
	}
}

func TestGenerateInvalidParams(t *testing.T) {
	valid := fizzbuzz.Params{Int1: 3, Int2: 5, Limit: 10, Str1: "fizz", Str2: "buzz"}

	tests := []struct {
		name      string
		mutate    func(*fizzbuzz.Params)
		wantParam string
	}{
		{"zero int1", func(p *fizzbuzz.Params) { p.Int1 = 0 }, "int1"},
		{"negative int1", func(p *fizzbuzz.Params) { p.Int1 = -3 }, "int1"},
		{"zero int2", func(p *fizzbuzz.Params) { p.Int2 = 0 }, "int2"},
		{"negative int2", func(p *fizzbuzz.Params) { p.Int2 = -5 }, "int2"},
		{"zero limit", func(p *fizzbuzz.Params) { p.Limit = 0 }, "limit"},
		{"negative limit", func(p *fizzbuzz.Params) { p.Limit = -1 }, "limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := valid
			tt.mutate(&params)

			got, err := fizzbuzz.Generate(params)

			var paramErr *fizzbuzz.ParamError
			if !errors.As(err, &paramErr) {
				t.Fatalf("Generate() error = %v, want a *ParamError", err)
			}
			if paramErr.Param != tt.wantParam {
				t.Errorf("ParamError.Param = %q, want %q", paramErr.Param, tt.wantParam)
			}
			if got != nil {
				t.Errorf("Generate() = %v, want nil on error", got)
			}
		})
	}
}
