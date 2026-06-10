package main

import (
	"reflect"
	"testing"
)

func TestSplitShellWords(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"simple", "npm run migrate", []string{"npm", "run", "migrate"}},
		{"empty", "", nil},
		{"whitespace only", "   \t ", nil},
		{"collapses runs of spaces", "a   b\t\tc", []string{"a", "b", "c"}},
		{
			"double quoted argument",
			`rails runner "User.count"`,
			[]string{"rails", "runner", "User.count"},
		},
		{
			"single quotes inside double quotes",
			`rails runner "User.where(name: 'x').count"`,
			[]string{"rails", "runner", "User.where(name: 'x').count"},
		},
		{
			"double quotes inside single quotes",
			`sh -c 'echo "hi there"'`,
			[]string{"sh", "-c", `echo "hi there"`},
		},
		{
			"escaped space outside quotes",
			`run my\ file`,
			[]string{"run", "my file"},
		},
		{
			"escaped double quote inside double quotes",
			`echo "say \"hi\""`,
			[]string{"echo", `say "hi"`},
		},
		{
			"backslash before regular char in double quotes stays literal",
			`grep "a\nb"`,
			[]string{"grep", `a\nb`},
		},
		{
			"adjacent quoted and unquoted segments form one word",
			`--flag="a b"c`,
			[]string{`--flag=a bc`},
		},
		{
			"empty quoted string is a word",
			`cmd ""`,
			[]string{"cmd", ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitShellWords(tc.input)
			if err != nil {
				t.Fatalf("splitShellWords(%q): %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitShellWords(%q) = %#v, want %#v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSplitShellWords_Errors(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"unterminated single quote", `echo 'oops`},
		{"unterminated double quote", `echo "oops`},
		{"trailing backslash", `echo oops\`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := splitShellWords(tc.input); err == nil {
				t.Errorf("splitShellWords(%q): expected error, got nil", tc.input)
			}
		})
	}
}
