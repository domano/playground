package internal

import (
	"context"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Serve(url string) {
	// Listen for SIGINT and SIGTERM
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	storage := memory.NewStorage()
	fileSystem := memfs.New()
	_, err := git.Clone(storage, fileSystem, &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	})
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(NewFileSystemConnector(fileSystem))

	server := http.Server{
		Addr:    ":8080",
		Handler: fileServer,
	}
	go func() {
		if err := http.ListenAndServe(":8080", fileServer); err != nil {
			log.Fatal(err)
		}
	}()

	log.Printf("server running")

	<-sigs
	log.Printf("shutting down server")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err = server.Shutdown(ctxShutDown); err != nil {
		log.Fatalf("server Shutdown Failed:%+s", err)
	}

	log.Printf("server exited properly")
}

//func SSHClone() {
//	CheckArgs("<url>", "<directory>", "<private_key_file>")
//	url, directory, privateKeyFile := os.Args[1], os.Args[2], os.Args[3]
//	var password string
//	if len(os.Args) == 5 {
//		password = os.Args[4]
//	}
//
//	_, err := os.Stat(privateKeyFile)
//	if err != nil {
//		Warning("read file %s failed %s\n", privateKeyFile, err.Error())
//		return
//	}
//
//	// Clone the given repository to the given directory
//	Info("git clone %s ", url)
//	publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKeyFile, password)
//	if err != nil {
//		Warning("generate publickeys failed: %s\n", err.Error())
//		return
//	}
//
//	r, err := git.PlainClone(directory, false, &git.CloneOptions{
//		// The intended use of a GitHub personal access token is in replace of your password
//		// because access tokens can easily be revoked.
//		// https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
//		Auth:     publicKeys,
//		URL:      url,
//		Progress: os.Stdout,
//	})
//	CheckIfError(err)
//
//	// ... retrieving the branch being pointed by HEAD
//	ref, err := r.Head()
//	CheckIfError(err)
//	// ... retrieving the commit object
//	commit, err := r.CommitObject(ref.Hash())
//	CheckIfError(err)
//
//	fmt.Println(commit)
//}
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
		valid := fs.ValidPath(name[1:]) // ignore leading slash from url, since we are not working with a full filesystem and dont refer to root with it
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
