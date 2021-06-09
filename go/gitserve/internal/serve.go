package internal

import (
	"context"
	"errors"
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

func Serve(ctx context.Context, ctxCancel context.CancelFunc, opts *git.CloneOptions, interval time.Duration) {
	// Listen for SIGINT and SIGTERM
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	storage := memory.NewStorage()
	fileSystem := memfs.New()
	repo, err := git.Clone(storage, fileSystem, opts)
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

	go keepRepoUpdated(ctx, repo, opts, interval)

	<-sigs
	log.Printf("shutting down server")

	ctxShutDown, _ := context.WithTimeout(ctx, 5*time.Second) // We will use the parents cancellation func instead
	defer func() {
		ctxCancel() // We trigger the parents' cancellation func so as to notify the worktree update routine of the cancellation too
	}()

	if err = server.Shutdown(ctxShutDown); err != nil {
		log.Fatalf("server Shutdown Failed:%+s", err)
	}

	log.Printf("server exited properly")
}

func keepRepoUpdated(ctx context.Context, repo *git.Repository, cloneOptions *git.CloneOptions, interval time.Duration) {
	worktree, err := repo.Worktree()
	if errors.Is(err, git.ErrIsBareRepository) {
		log.Println("Can not update worktrees for bare repositories, no updates will take place")
		return
	}
	if err != nil {
		log.Fatalf("Encountered an unexpected error whilst getting the worktree for the repo: %s\n", err)
	}
	fetchOptions := fetchOpts(cloneOptions)
	pullOptions := pullOpts(cloneOptions)

	for ctx.Err() == nil {
		err := repo.FetchContext(ctx, fetchOptions)
		if !errors.Is(err, git.NoErrAlreadyUpToDate) && err != nil {
			log.Printf("Could not fetch remote: %s\n", err)
		}
		if err == nil { // We should only try to update the worktree if there was no git.NoErrAlreadyUpToDate
			log.Println("Updating Worktree")
			err := worktree.PullContext(ctx, pullOptions)
			if err != nil {
				log.Printf("Could not pull in new changes: %s\n", err)
			} else {
				log.Println("Updated the worktree")
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			continue
		}
	}
}

func pullOpts(opts *git.CloneOptions) *git.PullOptions {
	return &git.PullOptions{
		RemoteName:        opts.RemoteName,
		ReferenceName:     opts.ReferenceName,
		SingleBranch:      opts.SingleBranch,
		Depth:             opts.Depth,
		Auth:              opts.Auth,
		RecurseSubmodules: opts.RecurseSubmodules,
		Progress:          opts.Progress,
		Force:             true,
		InsecureSkipTLS:   opts.InsecureSkipTLS,
		CABundle:          opts.CABundle,
	}
}

func fetchOpts(opts *git.CloneOptions) *git.FetchOptions {
	return &git.FetchOptions{
		RemoteName:      opts.RemoteName,
		Depth:           opts.Depth,
		Auth:            opts.Auth,
		Progress:        opts.Progress,
		Tags:            opts.Tags,
		Force:           true,
		InsecureSkipTLS: opts.InsecureSkipTLS,
		CABundle:        opts.CABundle,
	}
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
