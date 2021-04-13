package cmd

import (
	"context"
	"fmt"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gitserve",
	Short: "Serve any git repository from memory via http.",
	Long:  `Serve any git repository from memory via http.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.Help()
			return
		}
		url := args[0]

		// if we do not have something like a protocol specifier in front or it does not look like a ssh-url we append https:// as a hack
		match, err := regexp.Match(".*(://|@.*:).*", []byte(url))
		if err != nil {
			cmd.PrintErr(err)
			return
		}
		if !match {
			url = "https://" + url
		}
		Serve(url) // TODO: add auth for ssh clone
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gitserve.yaml)")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".gitserve" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".gitserve")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

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
