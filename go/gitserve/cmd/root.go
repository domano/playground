package cmd

import (
	"fmt"
	"github.com/domano/playground/go/gitserve/internal"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

var privateKey string
var privateKeyPassword bool
var url string

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
		url = args[0]

		// if we do not have something like a protocol specifier in front or it does not look like a ssh-url we append https:// as a hack
		match, err := regexp.Match(".*(://|@.*:).*", []byte(url))
		if err != nil {
			cmd.PrintErr(err)
			return
		}
		if !match {
			url = "https://" + url
		}

		usr, _ := user.Current()
		dir := usr.HomeDir
		if privateKey == "~" {
			// In case of "~", which won't be caught by the "else if"
			privateKey = dir
		} else if strings.HasPrefix(privateKey, "~/") {
			// Use strings.HasPrefix so we don't match paths like
			// "/something/~/something/"
			privateKey = filepath.Join(dir, privateKey[2:])
		}

		var pw string
		if privateKeyPassword {
			pw, err = password()
			if err != nil {
				panic(err)
			}
		}
		publicKeys, err := ssh.NewPublicKeysFromFile("git", privateKey, pw)
		if err != nil {
			panic(err)
		}

		opts := git.CloneOptions{
			URL:               url,
			Auth:              publicKeys,
			RemoteName:        "",
			ReferenceName:     "",
			SingleBranch:      false,
			NoCheckout:        false,
			Depth:             0,
			RecurseSubmodules: 0,
			Progress:          nil,
			Tags:              0,
			InsecureSkipTLS:   false,
			CABundle:          nil,
		}

		internal.Serve(&opts)
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

	rootCmd.PersistentFlags().StringVarP(&privateKey, "privateKey", "k", "~/.ssh/id_rsa", "Path to SSH private key. By default ~/.ssh/id_rsa will be used if a ssh:// repo is passed.")
	rootCmd.PersistentFlags().BoolVarP(&privateKeyPassword, "privateKeyPassword", "p", false, "Is the SSH Key password protected?")

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

func password() (string, error) {
	fmt.Print("Enter Password: ")
	bytePassword, err := terminal.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", err
	}

	password := string(bytePassword)
	return strings.TrimSpace(password), nil
}
