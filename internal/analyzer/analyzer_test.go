package analyzer

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()

	// Тестируем пакет без нарушений
	analysistest.Run(t, testdata, Analyzer, "good")

	// Тестируем пакет с panic
	analysistest.Run(t, testdata, Analyzer, "panic")

	// Тестируем пакет с log.Fatal/os.Exit вне main.main
	analysistest.Run(t, testdata, Analyzer, "fatal")

	// Тестируем пакет main с разрешённым log.Fatal в main.main и нарушением в helper
	analysistest.Run(t, testdata, Analyzer, "mainpkg")
}
