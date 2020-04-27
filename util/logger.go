package util

import (
	"bytes"
	"encoding/json"
	"fmt"
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
}

func PrintPrettyJson(prefix string, data []byte) {
	var prettyJSON bytes.Buffer
	error := json.Indent(&prettyJSON, data, "", "\t")
	if error != nil {
		ConsoleLogger.Println("JSON parse error: ", error)
		return
	}
	ConsoleLogger.Println(fmt.Sprintf("%s%s", prefix, string(prettyJSON.Bytes())))

}
