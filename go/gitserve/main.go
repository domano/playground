package main

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"os"
)

func main() {
	storage := memory.NewStorage()
	git.Clone(storage, nil, &git.CloneOptions{
		URL:      "https://github.com/domano/playground",
		Progress: os.Stdout,
	})

	iter, _ := storage.IterEncodedObjects(plumbing.BlobObject)
	print(iter)
}
