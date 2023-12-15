package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	masa "github.com/masa-finance/masa-oracle/pkg"
	"github.com/masa-finance/masa-oracle/pkg/crypto"
)

func init() {
	f, err := os.OpenFile("masa_node_lite.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		logrus.Fatal(err)
	}
	mw := io.MultiWriter(os.Stdout, f)
	logrus.SetOutput(mw)
	logrus.SetLevel(logrus.InfoLevel)

	usr, err := user.Current()
	if err != nil {
		logrus.Fatal("could not find user.home directory")
	}
	envFilePath := filepath.Join(usr.HomeDir, ".masa", "masa_oracle_node.env")
	keyFilePath := filepath.Join(usr.HomeDir, ".masa", "masa_oracle_key")

	// Create the directories if they don't already exist
	if _, err := os.Stat(filepath.Dir(envFilePath)); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(envFilePath), 0755)
		if err != nil {
			logrus.Fatal("could not create directory:", err)
		}
	}
	// Check if the .env file exists
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {
		// If not, create it with default values
		builder := strings.Builder{}
		builder.WriteString(fmt.Sprintf("%s=%s\n", masa.KeyFileKey, keyFilePath))
		err = os.WriteFile(envFilePath, []byte(builder.String()), 0644)
		if err != nil {
			logrus.Fatal("could not write to .env file:", err)
		}
	}
	err = godotenv.Load(envFilePath)
	if err != nil {
		logrus.Error("Error loading .env file")
	}
}

func main() {
	logrus.Infof("arg size is %d", len(os.Args))
	if len(os.Args) > 1 {
		logrus.Infof("found arg: %s", os.Args[1])
		err := os.Setenv(masa.Peers, os.Args[1])
		if err != nil {
			logrus.Error(err)
		}
		if len(os.Args) == 3 {
			err := os.Setenv(masa.PortNbr, os.Args[2])
			if err != nil {
				logrus.Error(err)
			}
		}
	}
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Listen for SIGINT (CTRL+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Cancel the context when SIGINT is received
	go func() {
		<-c
		cancel()
	}()

	privKey, _, _, err := crypto.GetOrCreatePrivateKey(os.Getenv(masa.KeyFileKey))
	if err != nil {
		logrus.Fatal(err)
	}
	node, err := masa.NewOracleNode(ctx, privKey, getPort(masa.PortNbr), true, true, false)
	if err != nil {
		logrus.Fatal(err)
	}

	node.Start()
	<-ctx.Done()
}

func getPort(name string) int {
	valueStr := os.Getenv(name)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return 0
}
