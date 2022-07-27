// utils.go
package utils

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"runtime"
)

func FatalDetails() string {
	_, file, line, _ := runtime.Caller(2)
	s := fmt.Sprintf("At: %v:l.%v", file, line)
	return s
}

func ErrDefaultFatal(err error) {
	if err != nil {
		err = fmt.Errorf("%w; %v", err, FatalDetails())
		log.Fatal(err)
	}

}

func StringCleaned(raw []byte) string {
	//convert to string, remove newline characters
	s := string(raw[:])
	re := regexp.MustCompile(`\r?\n`)
	s = re.ReplaceAllString(s, "")

	return s
}

func CheckFileExists(pathFile string) bool {

	if _, err := os.Stat(pathFile); err == nil {
		//  exists
		return true
	} else if errors.Is(err, os.ErrNotExist) {
		// does not exist
		return false
	} else {
		// file may or may not exist. See err for details.
		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		log.Println("Strange state of file: " + pathFile + ":" + FatalDetails())
		return false
	}

}

// read last line; if tNonEmpty, last non-empty line
func GetLastLine(pathFile string, tNonEmpty bool) string {
	lastLine := ""
	lastNewLineChar := make([]byte, 1)
	tFound := false

	tExists := CheckFileExists(pathFile)
	if !tExists {
		return lastLine //the empty string
	}

	file, err := os.Open(pathFile)
	ErrDefaultFatal(err)
	defer file.Close()

	var cur int64 = 0
	stat, _ := file.Stat()
	filesize := stat.Size()
	for {
		cur -= 1
		file.Seek(cur, io.SeekEnd)

		char := make([]byte, 1)
		file.Read(char)

		if char[0] != 10 && char[0] != 13 {
			tFound = true
		}

		if tNonEmpty {
			if char[0] == 10 || char[0] == 13 {
				//is this a newline after some content
				if tFound {
					lastLine = fmt.Sprintf("%s%s", lastLine, string(lastNewLineChar)) //append the old newline
					break
				}

				//keep the old newline
				lastNewLineChar = char
			} else {
				lastLine = fmt.Sprintf("%s%s", string(char), lastLine) // prepend the char
			}
		} else {
			//keep also empty lines
			if cur != -1 && (char[0] == 10 || char[0] == 13) { // stop if we find a line
				break
			}
			lastLine = fmt.Sprintf("%s%s", string(char), lastLine) // prepend the char

		}

		if cur == -filesize { // stop if we are at the begining
			break
		}
	}

	return lastLine
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
