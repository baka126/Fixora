package server

import (
	"errors"
	"testing"
)

func TestIsUndefinedTableError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "postgres sqlstate", err: errors.New(`ERROR: relation "users" does not exist (SQLSTATE 42P01)`), want: true},
		{name: "plain relation message", err: errors.New(`relation "users" does not exist`), want: true},
		{name: "other error", err: errors.New("connection refused"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUndefinedTableError(tc.err); got != tc.want {
				t.Fatalf("isUndefinedTableError() = %v, want %v", got, tc.want)
			}
		})
	}
}
