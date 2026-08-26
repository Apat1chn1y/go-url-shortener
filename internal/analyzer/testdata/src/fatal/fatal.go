package fatal

import (
	"log"
	"os"
)

func helper() {
	log.Fatal("bad") // want "call to log.Fatal outside main.main"
	os.Exit(1)       // want "call to os.Exit outside main.main"
}
