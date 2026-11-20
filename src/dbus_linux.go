package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"tripod311/faraday/vault"

	"github.com/godbus/dbus/v5"
	"golang.org/x/term"
)

const (
	serviceName = "io.github.Tripod311.Faraday"
	objectPath  = dbus.ObjectPath("/io/github/Tripod311/Faraday")
)

type DBusConfig struct {
	VaultPath   string   `json:"vault_path"`
	AllowedKeys []string `json:"allowed_keys"`
}

type Faraday struct {
	secrets map[string]string
}

func (f *Faraday) ReadKey(key string) (string, *dbus.Error) {
	secret, exists := f.secrets[key]
	if !exists {
		return "", dbus.NewError(
			"io.github.Tripod311.Faraday.Error.KeyNotFound",
			[]any{"Secret key not found"},
		)
	}

	return secret, nil
}

func askPassword() ([]byte, error) {
	fmt.Fprint(os.Stderr, "Enter vault key: ")

	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)

	return password, err
}

func makeInstance(filename string) Faraday {
	var instance Faraday
	instance.secrets = make(map[string]string)

	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Can't open config: %w", err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("Can't read config: %w", err)
	}

	var config DBusConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Config is corrupted: %w", err)
	}

	f.Close()

	password, err := askPassword()
	if err != nil {
		log.Fatalf("Error on reading password: %w", err)
	}

	v_data, err := vault.ReadVault(config.VaultPath, password)
	if err != nil {
		log.Fatal(err)
	}

	var v vault.Vault

	err = v.Deserialize(v_data)
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < len(config.AllowedKeys); i++ {
		key := config.AllowedKeys[i]
		value, err := v.GetField(key)
		if err != nil {
			log.Printf("Key %s is missing, skipping")
			continue
		}
		instance.secrets[key] = value
	}

	return instance
}

func main() {
	filename := flag.String("config", "", "Path to config")
	flag.Parse()

	instance := makeInstance(*filename)

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatal(err)
	}

	if err := conn.Export(&instance, objectPath, serviceName); err != nil {
		log.Fatal(err)
	}

	reply, err := conn.RequestName(serviceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Fatal(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("D-Bus name already taken: %s", serviceName)
	}

	log.Printf("Listening on %s", serviceName)
	select {}
}
