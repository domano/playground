package main

import (
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Listen for SIGINT and SIGTERM
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		done <- true
	}()

	storage := memory.NewStorage()
	fileSystem := memfs.New()
	_, err := git.Clone(storage, fileSystem, &git.CloneOptions{
		URL:      "https://github.com/domano/playground",
		Progress: os.Stdout,
	})
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(NewFileSystemConnector(fileSystem))

	if err := http.ListenAndServe(":8080", fileServer); err != nil {
		panic(err)
	}

	<-done

}

type FileSystemConnector struct {
	mfs billy.Filesystem
}

func NewFileSystemConnector(mfs billy.Filesystem) FileSystemConnector {
	return FileSystemConnector{
		mfs: mfs,
	}
}

func (f FileSystemConnector) Open(name string) (http.File, error) {
	if name == "/" {
		name = "index.html"
	} else {
		valid := fs.ValidPath(name)
		if !valid {
			return nil, fs.ErrNotExist
		}
	}
	file, err := f.mfs.Open(name)
	return FileConnector{f.mfs, file, name}, err
}

type FileConnector struct {
	billy.Filesystem
	billy.File
	string
}

// Readdir reads all contents regardless of count given, due to the underlying billy.Filesystem. It will not return more then count slice entries.
func (f FileConnector) Readdir(count int) ([]fs.FileInfo, error) {
	fi, err := f.Filesystem.ReadDir(f.string)
	return fi[0:count], err // only return an array with as many elements as in "count"
}

func (f FileConnector) Stat() (fs.FileInfo, error) {
	return f.Filesystem.Stat(f.Name())
}
