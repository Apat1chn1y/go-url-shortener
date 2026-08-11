package good

import "log"

func main() {
	log.Println("hello")
}

func helper() {
	// no panic, no log.Fatal, no os.Exit
}
