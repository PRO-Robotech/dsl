package generator

import "os"

func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
