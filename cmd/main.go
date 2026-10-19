package main

import (
	"codeknight03/storaga/pkg/fops"
	"fmt"
	"os"
)

func main() {

	// start a file
	fd, err := os.OpenFile("kvstore.bin", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0660)
	if err != nil {
		fmt.Printf("Couldn't open the specified file")
		return

	}

	err = fops.Put([]byte("Kapil"), []byte("Ramwani"), *fd)
	if err != nil {
		fmt.Printf("Record append failed with : %s", err)
	}

	fd.Close()
}
