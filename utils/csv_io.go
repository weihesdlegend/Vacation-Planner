package utils

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
)

// ReadCsv ...
// input a file name and get a slice of string slices...
func ReadCsv(filename string) (res [][]string) {
	if !checkExist(filename) {
		return
	}
	csvFile, err := os.Open(filename)
	LogErrorWithLevel(err, LogFatal)

	defer csvFile.Close()

	reader := csv.NewReader(bufio.NewReader(csvFile))
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		} else {
			res = append(res, line)
		}
	}
	return
}

// WriteCsv...
// write a slice of string slices to specified file
func WriteCsv(filename string, records [][]string) {
	if !checkExist(filename) {
		_, err := os.Create(filename)
		LogErrorWithLevel(err, LogFatal)
	}
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	LogErrorWithLevel(err, LogFatal)

	// close file after writing is done
	defer file.Close()

	writer := csv.NewWriter(file)
	// flush before closing the file
	defer writer.Flush()

	for _, line := range records {
		err := writer.Write(line)
		LogErrorWithLevel(err, LogFatal)
	}
}

func checkExist(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir() // false if directory
}
