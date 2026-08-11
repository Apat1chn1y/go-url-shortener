package main

import "log"

func main() {
	log.Fatal("ok") // разрешено в main.main
}

func helper() {
	log.Fatal("bad") // want "call to log.Fatal outside main.main"
}
