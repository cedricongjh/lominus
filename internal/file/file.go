// Package file provides util primitives to file operations.
package file

import (
	"encoding/gob"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// EncodeToFile takes in any type and encodes it into a file specified by fileName.
// If the file already exists, it is truncated.
// If the file does not exist, it is created with mode 0666 (before umask).
// Provide absolute path else file will be written to the current working directory.
// TODO CHANGE name to EncodeToFile
func EncodeStructToFile(fileName string, data interface{}) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := gob.NewEncoder(file)
	encodeErr := encoder.Encode(data)
	if encodeErr != nil {
		return encodeErr
	}

	return nil
}

// DecodeFromFile decodes any type that has been encoded back into its original type.
// Works with structs, primitives and objects like time.Time.
// Provide a pointer of the type to write to.
// Provide absolute path else file may not be found.
// TODO CHANGE name to DecodeToFile
func DecodeStructFromFile(fileName string, data interface{}) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	decodeErr := decoder.Decode(data)
	if decodeErr != nil {
		return decodeErr
	}

	return nil
}

// Exists checks if the given file exists.
func Exists(name string) bool {
	_, err := os.Stat(name)

	return err == nil
}

// AutoRename renames a file by prepending "[vX]" to its fileName
// where X is a positive integer.
// X increments itself starting from 1 until the there exists a
// the new fileName does not exist in the directory.
func AutoRename(filePath string) error {
	FORMAT := "[v%d]%s"
	directory, fileName := filepath.Split(filePath)
	newFileName := fileName

	for x := 1; ; x++ {
		newFileName = fmt.Sprintf(FORMAT, x, fileName)

		if !Exists(filepath.Join(directory, newFileName)) {
			break
		}
	}

	return os.Rename(filePath, filepath.Join(directory, newFileName))
}

// EnsureDir is a helper function that ensures that the directory exists by creating them
// if they do not already exist.
func EnsureDir(dir string) error {
	if _, serr := os.Stat(dir); serr != nil {
		merr := os.MkdirAll(dir, os.ModePerm)
		if merr != nil {
			return merr
		}
	}

	return nil
}

// CleanseFolderFileName is a helper function that ensures folders' and files' names are valid,
// that they do not contain prohibited characters. However, some are still not caught for
// unlikeliness and simplicity reasons.
// The following are reserved file names for Windows that are uncaught:
// CON, PRN, AUX, NUL, COM1, COM2, COM3, COM4, COM5, COM6, COM7, COM8, COM9, LPT1, LPT2, LPT3, LPT4, LPT5, LPT6, LPT7, LPT8, LPT9.
// The following are non-printable characters that are uncaught:
// ASCII 0-31.
func CleanseFolderFileName(name string) string {
	name = strings.Replace(name, "/", " ", -1)
	name = strings.Replace(name, "\\", " ", -1)
	name = strings.Replace(name, "<", " ", -1)
	name = strings.Replace(name, ">", " ", -1)
	name = strings.Replace(name, ":", " ", -1)
	name = strings.Replace(name, "\"", " ", -1)
	name = strings.Replace(name, "|", " ", -1)
	name = strings.Replace(name, "?", " ", -1)
	name = strings.Replace(name, "*", " ", -1)
	name = strings.TrimSpace(name)

	// We can ignore the error (if any) because it would just mean
	// that the "%XX" that appears in the name is legit, and not
	// because of URL encoding.
	name, _ = url.QueryUnescape(name)

	return name
}

// FileNotFoundError struct is an error struct that contains the custom error that will be thrown when file is not found.
type FileNotFoundError struct {
	FileName string
}

// FileNotFoundError is an error that will be thrown when file is not found.
func (e *FileNotFoundError) Error() string {
	return fmt.Sprintf("FileNotFoundError: %s cannot be found.", e.FileName)
}
