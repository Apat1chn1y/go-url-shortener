// Package main запускает статический анализатор с использованием singlechecker.
package main

import (
	"github.com/Apat1chn1y/go-url-shortener.git/internal/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
