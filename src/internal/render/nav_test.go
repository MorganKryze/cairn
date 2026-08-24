package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MorganKryze/cairn/src/internal/config"
	"github.com/MorganKryze/cairn/src/internal/testutil"
)

func TestCategoryNavScales(t *testing.T) {
	build := func(n int) string {
		var sb strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&sb, "- {id: s%d, url: https://s%d.example.org, name: S%d, category: c%02d}\n", i, i, i, i)
		}
		cfg, err := config.Load(testutil.WriteFiles(t, map[string]string{"services.yaml": sb.String()}))
		if err != nil {
			t.Fatal(err)
		}
		m, err := BuildModel(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		return string(m.Pages["en"].HTML)
	}

	// One control at every count. The trail used to split at seven into a
	// wrapping chip row and a jump-to select; the select drove nothing without
	// JavaScript, so it had to be swapped in behind data-js, and it never said
	// which category the reader was in. The row scrolls sideways instead.
	for _, n := range []int{2, 4, 8, 20} {
		h := build(n)
		if strings.Contains(h, "toc-select") || strings.Contains(h, "toc-jump") {
			t.Errorf("%d categories brought back the jump-to select", n)
		}
		if strings.Contains(h, "toc many") {
			t.Errorf("%d categories still carry the .many modifier", n)
		}
		if got := strings.Count(h, `<nav class="toc"`); got != 1 {
			t.Errorf("%d categories rendered %d trails, want exactly 1", n, got)
		}
		// every category reachable, since a row that scrolls hides some of them
		// off screen and a missing entry would look like one scrolled away
		if got := strings.Count(h, `<li><a href="#cat-`); got != n {
			t.Errorf("%d categories listed %d entries in the trail", n, got)
		}
	}

	// Nothing to navigate between with a single category.
	if h := build(1); strings.Contains(h, `class="toc"`) {
		t.Error("one category should render no trail at all")
	}
}
