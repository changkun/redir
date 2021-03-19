// Copyright 2021 Changkun Ou. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package utils_test

import (
	"strings"
	"testing"

	"changkun.de/x/redir/internal/utils"
)

func TestRandomString(t *testing.T) {
	s1 := utils.Randstr(12)
	s2 := utils.Randstr(12)
	if len(s1) != 12 || len(s2) != 12 {
		t.Fatalf("want 12 chars, got: %v, %v", len(s1), len(s2))
	}
	if strings.Compare(s1, s2) == 0 {
		t.Fatalf("want two different string, got: %v, %v", s1, s2)
	}
	t.Log(s1, s2)
}
