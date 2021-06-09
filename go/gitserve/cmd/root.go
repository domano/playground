package cmd

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var privateKey string
var addr string
var updateInterval time.Duration
var url string

// TODO: make this work for all subcommands
var baseCtx, baseCtxCancel = context.WithCancel(context.Background())

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gitserve",
	Short: "Serve any git repository from memory via http.",
	Long:  `Serve any git repository from memory via http whilst keeping it up to date.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {

	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&privateKey, "privateKey", "k", "~/.ssh/id_rsa", "For SSH cloning, fetching and pulling you can pass a private key.")
	rootCmd.PersistentFlags().StringVarP(&addr, "address", "a", ":8080", "Address to use for the server.")
	rootCmd.PersistentFlags().DurationVarP(&updateInterval, "updateInterval", "u", 5*time.Minute, "Interval that determines how often we check and pull in changes from git.")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if privateKey != "" {
		// Use config file from the flag.
		viper.SetConfigFile(privateKey)
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

func getPublicKey(privateKey string) *ssh.PublicKeys {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if privateKey == "~" {
		// In case of "~", which won't be caught by the prefix case
		privateKey = dir
	}
	if strings.HasPrefix(privateKey, "~/") {
		// Use strings.HasPrefix so we don't match paths like
		// "/something/~/something/"
		privateKey = filepath.Join(dir, privateKey[2:])
	}

	publicKeys, err := checkPassword(ssh.NewPublicKeysFromFile("git", privateKey, "pw"))
	if err != nil {
		log.Fatal(err)
	}
	return publicKeys
}

func checkPassword(publicKeys *ssh.PublicKeys, err error) (*ssh.PublicKeys, error) {
	if errors.Is(err, x509.IncorrectPasswordError) || (err != nil && strings.Contains(err.Error(), "password")) { // hacky catch-all check for passwords since not all possible password errors are properly typed
		pw, err := password()
		if err != nil {
			return nil, err
		}
		return checkPassword(ssh.NewPublicKeysFromFile("git", privateKey, pw))
	}
	return publicKeys, err
}

func password() (string, error) {
	fmt.Println("Enter Password: ")
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}

	password := string(bytePassword)
	return strings.TrimSpace(password), nil
}
