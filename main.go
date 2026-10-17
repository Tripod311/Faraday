package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"tripod311/faraday/ui"
	"tripod311/faraday/vault"

	"fyne.io/fyne/v2"
	"golang.org/x/term"
)

func askPassword() ([]byte, error) {
	fmt.Fprint(os.Stderr, "Enter vault key: ")

	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)

	return password, err
}

func main() {
	filename := flag.String("vault", "secrets.vault", "Path to vault")
	field := flag.String("field", "", "Field name in vault, for example mp/devKeys")
	value := flag.String("content", "", "Field content; defaults to stdin")
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		ui := ui.CreateUI()

		ui.MainWindow.Resize(fyne.NewSize(600, 600))
		ui.MainWindow.ShowAndRun()
	} else {
		password, err := askPassword()

		if err != nil {
			log.Fatalf("Error on reading vault password")
		}

		// try to execute operation on vault
		switch args[0] {
		case "create":
			err := vault.CreateVault(*filename, password)

			if err != nil {
				log.Fatal(err)
			}
		case "read":
			content, err := vault.ReadVault(*filename, password)
			if err != nil {
				log.Fatal(err)
			}

			v := vault.Vault{}
			err = v.Deserialize(content)
			if err != nil {
				log.Fatal(err)
			}

			fieldContent, err := v.GetField(*field)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Print(fieldContent)
		case "update":
			content, err := vault.ReadVault(*filename, password)
			if err != nil {
				log.Fatal(err)
			}

			v := vault.Vault{}
			err = v.Deserialize(content)
			if err != nil {
				log.Fatal(err)
			}

			valueProvided := false
			flag.Visit(func(f *flag.Flag) {
				if f.Name == "content" {
					valueProvided = true
				}
			})

			fieldContent := *value

			if !valueProvided {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					log.Fatal(err)
				}
				fieldContent = string(data)
			}

			err = v.SetField(*field, fieldContent)
			if err != nil {
				log.Fatal(err)
			}

			updated, err := v.Serialize()
			if err != nil {
				log.Fatal(err)
			}

			err = vault.WriteVault(*filename, password, updated)
			if err != nil {
				log.Fatal(err)
			}
		case "pwd":
			content, err := vault.ReadVault(*filename, password)
			if err != nil {
				log.Fatal(err)
			}

			err = vault.WriteVault(*filename, password, content)

			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
