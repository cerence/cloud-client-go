package util

import (
	"log"
	"os"
)

var (
	ConsoleLogger *log.Logger // Important information
	FileLogger    *log.Logger // Be concerned

)

func init() {

	ConsoleLogger = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Lmicroseconds|log.Lshortfile)

	FileLogger = log.New(os.Stdout,
		"INFO: ",
		log.Ldate|log.Lmicroseconds|log.Lshortfile)

}
