package main

import (
	"log"
	"time"

	"github.com/kitech/gopp"
	"github.com/qtui/qtsyms"
)

func main() {
	stub := "engine"
	var nowt = time.Now()
	signt := qtsyms.LoadAllQtSymbols(stub)
	log.Println(gopp.Lenof(signt), time.Since(nowt)) // about 1.1s

}
