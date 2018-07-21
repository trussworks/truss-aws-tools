package main

import (
	"testing"
	"time"
)

func TestParseTimestamp(t *testing.T) {

	cases := []struct {
		in   string
		want time.Time
	}{
		{"2018-07-11T19:19:08+00:00", time.Date(2018, 7, 11, 19, 19, 8, 0, time.UTC)},
		{"2018-06-25T19:05:23+00:00", time.Date(2018, 6, 25, 19, 05, 23, 0, time.UTC)},
	}
	for _, c := range cases {
		got, err := parseTimestamp(c.in)
		if err != nil {
			t.Errorf("parseTimestamp(%q) through an error %q", c.in, err)
		}
		if !got.Equal(c.want) {
			t.Errorf("parseTimestamp(%q) == %q, want %q", c.in, got, c.want)
		}
	}
}
