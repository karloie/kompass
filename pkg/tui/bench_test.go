package tui

import (
	"fmt"
	"strings"
	"testing"

	kube "github.com/karloie/kompass/pkg/kube"
	"github.com/karloie/kompass/pkg/mock"
	"github.com/karloie/kompass/pkg/pipeline"
)

func BenchmarkHighlightResourceLineYAML(b *testing.B) {
	line := "metadata: {name: petshop-tennant, namespace: petshop, labels: {app: tennant}}"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = highlightResourceLine("yaml", line)
	}
}

func BenchmarkHighlightResourceLineLogsStructured(b *testing.B) {
	line := "2026-03-03 11:26:24.421+0000 INFO Logging config in use: File '/var/lib/neo4j/conf/user-logs.xml'"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = highlightResourceLine("logs", line)
	}
}

func BenchmarkHighlightResourceLineDescribe(b *testing.B) {
	line := "Namespace: petshop"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = highlightResourceLine("describe", line)
	}
}

func BenchmarkRenderRowsFileOutput(b *testing.B) {
	m := newRun(Options{Mode: ModeSelector})
	m.width = 140
	m.height = 40
	m.view = &View{Kind: FileOutput, Rows: make([]string, 0, 2000)}
	for i := 0; i < 2000; i++ {
		m.view.Rows = append(m.view.Rows, fmt.Sprintf("line=%d key=value component=petshop-tennant", i))
	}
	m.view.Scroll = 400
	m.view.ColScroll = 6
	rowsHeight := 30

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderRows(rowsHeight)
	}
}

func BenchmarkApplyMainFilterRows(b *testing.B) {
	m := newRun(Options{Mode: ModeSelector})
	rows := make([]Row, 0, 3000)
	for i := 0; i < 3000; i++ {
		row := Row{
			Key:       fmt.Sprintf("pod/petshop/petshop-tennant-%d", i),
			Type:      "pod",
			Name:      fmt.Sprintf("petshop-tennant-%d", i),
			Status:    "Running",
			Text:      fmt.Sprintf("pod petshop-tennant-%d namespace=petshop", i),
			Plain:     fmt.Sprintf("pod petshop-tennant-%d namespace=petshop", i),
			PlainText: fmt.Sprintf("pod petshop-tennant-%d namespace=petshop", i),
			Metadata:  map[string]any{"namespace": "petshop", "orphaned": i%7 == 0},
		}
		row.SearchText = buildRowSearchText(row)
		rows = append(rows, row)
	}
	m.allRowsByPane[0] = rows
	m.allRowsByPane[1] = rows
	m.filterQuery = "pod !single=true namespace=petshop"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.applyMainFilter()
	}
}

func BenchmarkBuildQueryMatcher(b *testing.B) {
	query := strings.Repeat("pod* !failed namespace=petshop ", 4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildQueryMatcher(query)
	}
}

func loadPetshopFixture(b testing.TB) ([]Row, *kube.Response) {
	b.Helper()
	provider := kube.NewMockClient(mock.GenerateMock())
	resp, err := pipeline.InferGraphs(provider, []string{"*/petshop/*"})
	if err != nil || resp == nil {
		b.Fatalf("petshop fixture: %v", err)
	}
	return flattenTrees(resp), resp
}

func BenchmarkApplyMainFilterRowsFixture(b *testing.B) {
	rows, resp := loadPetshopFixture(b)
	m := newRun(Options{Mode: ModeSelector})
	m.sourceTrees = resp
	m.allRowsByPane[0] = rows
	m.allRowsByPane[1] = singleRows(rows)
	m.filterQuery = "pod !single=true namespace=petshop"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.applyMainFilter()
	}
}

func BenchmarkRenderRowsFileOutputFixture(b *testing.B) {
	rows, _ := loadPetshopFixture(b)
	m := newRun(Options{Mode: ModeSelector})
	m.width = 140
	m.height = 40
	m.view = &View{Kind: FileOutput, Rows: make([]string, 0, len(rows))}
	for _, r := range rows {
		if !r.Separator {
			m.view.Rows = append(m.view.Rows, r.Plain)
		}
	}
	rowsHeight := 30
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderRows(rowsHeight)
	}
}
