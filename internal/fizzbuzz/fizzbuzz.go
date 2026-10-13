// Package fizzbuzz implements the generalised fizz-buzz sequence.
package fizzbuzz

import "strconv"

// Params describes a fizz-buzz request.
type Params struct {
	Int1  int    `json:"int1"`
	Int2  int    `json:"int2"`
	Limit int    `json:"limit"`
	Str1  string `json:"str1"`
	Str2  string `json:"str2"`
}

// ParamError reports a parameter that makes a request invalid.
type ParamError struct {
	Param  string
	Reason string
}

func (e *ParamError) Error() string {
	return e.Param + " " + e.Reason
}

// Validate checks the invariants the sequence needs to be defined.
// It returns a *ParamError describing the first invalid parameter.
func (p Params) Validate() error {
	switch {
	case p.Int1 < 1:
		return &ParamError{Param: "int1", Reason: "must be greater than or equal to 1"}
	case p.Int2 < 1:
		return &ParamError{Param: "int2", Reason: "must be greater than or equal to 1"}
	case p.Limit < 1:
		return &ParamError{Param: "limit", Reason: "must be greater than or equal to 1"}
	}
	return nil
}

// Generate returns the numbers from 1 to p.Limit as strings, where multiples
// of p.Int1 are replaced by p.Str1, multiples of p.Int2 by p.Str2, and
// multiples of both by p.Str1 followed by p.Str2.
//
// The result holds p.Limit elements: callers exposed to untrusted input are
// expected to bound p.Limit themselves.
func Generate(p Params) ([]string, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	both := p.Str1 + p.Str2
	out := make([]string, p.Limit)
	for i := 1; i <= p.Limit; i++ {
		multipleOf1, multipleOf2 := i%p.Int1 == 0, i%p.Int2 == 0
		switch {
		case multipleOf1 && multipleOf2:
			out[i-1] = both
		case multipleOf1:
			out[i-1] = p.Str1
		case multipleOf2:
			out[i-1] = p.Str2
		default:
			out[i-1] = strconv.Itoa(i)
		}
	}
	return out, nil
}
