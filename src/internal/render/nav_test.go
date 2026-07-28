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

	// A handful of categories: the chip trail, no jump-to select.
	if h := build(4); strings.Contains(h, "toc-select") {
		t.Error("few categories should stay chips, not a select")
	}

	// Above the threshold the compact jump-to select takes over on mobile, and
	// the trail keeps its sidebar role tagged with the .many modifier.
	h := build(8)
	if !strings.Contains(h, `class="toc-select"`) {
		t.Error("8 categories should render the jump-to select")
	}
	if !strings.Contains(h, `class="toc many"`) {
		t.Error("8 categories should tag the trail as many")
	}
}
