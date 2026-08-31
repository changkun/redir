// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package models_test

import (
	"bufio"
	"flag"
	"os"
	"strings"
	"testing"

	"changkun.de/x/redir/internal/models"
)

var update = flag.Bool("update", false, "rewrite the classification golden file")

// TestClassifyGolden derives every user agent in a sample taken from
// production and compares the result against a recorded one.
//
// The sample is the 120 most frequent user agent strings on changkun.de,
// which cover the overwhelming majority of visits. A synthetic table test
// cannot show whether the classifier is right about the traffic that
// actually arrives; this can. The golden file is meant to be reviewed when
// it changes, since a change there is a change in what the stats count.
func TestClassifyGolden(t *testing.T) {
	uas := readLines(t, "testdata/ua_sample.tsv")

	var got strings.Builder
	for _, line := range uas {
		_, ua, _ := strings.Cut(line, "\t")
		v := &models.Visit{UA: ua}
		v.Derive()
		got.WriteString(strings.Join([]string{
			boolField(v.IsBot), v.Browser, v.OS, v.Device, ua,
		}, "\t"))
		got.WriteString("\n")
	}

	const golden = "testdata/ua_classified.tsv"
	if *update {
		if err := os.WriteFile(golden, []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v; run: go test ./internal/models -update", err)
	}
	if got.String() != string(want) {
		wantLines := strings.Split(string(want), "\n")
		gotLines := strings.Split(got.String(), "\n")
		for i := range min(len(wantLines), len(gotLines)) {
			if wantLines[i] != gotLines[i] {
				t.Errorf("line %d:\n want %v\n  got %v", i+1, wantLines[i], gotLines[i])
			}
		}
		t.Fatalf("classification changed; review the diff, then: go test ./internal/models -update")
	}
}

// TestClassifyGoldenBotShare guards the number the golden file exists to
// protect: how much of the recorded traffic is automated. UptimeRobot
// alone is 66,358 of 348,356 production visits, all of which the dashboard
// counts as a person today.
func TestClassifyGoldenBotShare(t *testing.T) {
	var bots, total int
	for _, line := range readLines(t, "testdata/ua_sample.tsv") {
		n, ua, _ := strings.Cut(line, "\t")
		count := atoi(t, n)
		v := &models.Visit{UA: ua}
		v.Derive()
		total += count
		if v.IsBot {
			bots += count
		}
	}
	if total == 0 {
		t.Fatal("empty sample")
	}
	share := float64(bots) / float64(total)
	t.Logf("bot share of sampled visits: %.1f%% (%d/%d)", share*100, bots, total)
	// The measured share is 72.6%: most of what redir records is not a
	// person. A large move in either direction means the classifier
	// changed its mind about common traffic, which is worth reviewing.
	if share < 0.6 || share > 0.85 {
		t.Fatalf("bot share %.1f%% is outside the expected range", share*100)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			out = append(out, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func boolField(b bool) string {
	if b {
		return "bot"
	}
	return "human"
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("unparseable count %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
