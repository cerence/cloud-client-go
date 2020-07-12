package main

import (
	config2 "cloud-client-go/config"
	. "cloud-client-go/http_v2_client"
	. "cloud-client-go/util"
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"os"
	"sync"
)

var (
	wg sync.WaitGroup
)

func main() {
	disLogo := displayLogo()
	app := &cli.App{
		Name:  "cloud-client-go",
		Usage: "Make a Cerence cloud request, such as ASR, TTS requests",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "input",
				Aliases:  []string{"i"},
				Usage:    "input file contains the request parameters",
				EnvVars:  []string{"INPUT"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "output file contains the returned response data from cloud",
				EnvVars: []string{"OUTPUT"},
			},
			&cli.StringFlag{
				Name:    "audio",
				Aliases: []string{"a"},
				Usage:   "audio file contains the returned audio data from cloud",
				EnvVars: []string{"AUDIO"},
			},
		},
		Action: func(context *cli.Context) error {
			config := config2.ReadConfig(context.String("input"))
			client := NewHttpV2Client(config.Host, config.Port, WithProtocol(config.Protocol), WithPath(config.Path), WithBoundary(config.GetBoundary()))
			if client == nil {
				ConsoleLogger.Fatalln("Can't new connection")
			}

			if err := client.Connect(); err != nil {
				ConsoleLogger.Fatalln("Can't connect to server")
			}
			defer client.Close()

			wg.Add(2)
			//send
			go func() {
				defer func() {
					if err := recover(); err != nil {
						ConsoleLogger.Println(err)
					}
				}()
				defer wg.Done()
				Send(client, config)
				ConsoleLogger.Println("Send done")
			}()

			//receive
			go func() {
				defer func() {
					if err := recover(); err != nil {
						ConsoleLogger.Println(err)
					}
				}()
				defer wg.Done()
				output := context.String("output")
				audio := context.String("audio")
				Receive(client, output, audio)
				ConsoleLogger.Println("Receive done")
			}()

			wg.Wait()
			ConsoleLogger.Println("Request Complete")
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil && !disLogo {
		ConsoleLogger.Fatal(err)
	}
}
func displayLogo() bool {
	if len(os.Args) == 1 {
		color.Cyan(logo)
		return true
	} else if len(os.Args) == 2 {
		if os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "h" || os.Args[1] == "help" {
			color.Cyan(logo)
			return true
		}
	}
	return false
}
