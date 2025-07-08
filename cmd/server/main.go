package main

import (
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
//	"os/signal"
	"github.com/trolioSFG/learn-pub-sub-starter/internal/pubsub"
	"github.com/trolioSFG/learn-pub-sub-starter/internal/routing"
	"github.com/trolioSFG/learn-pub-sub-starter/internal/gamelogic"
	"log"
)


func main() {
	fmt.Println("Starting Peril server...")
	url := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		fmt.Println("Error: %w", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("Connection success")

	gamelogic.PrintServerHelp()

	channel, err := conn.Channel()
	if err != nil {
		fmt.Println("Could not create channel:", err)
		os.Exit(1)
	}

	_, _, err = pubsub.DeclareAndBind(conn, "peril_topic", "game_logs", "game_logs.*", pubsub.Durable)
	if err != nil {
		fmt.Printf("Error DeclareAndBind game_logs: %v", err)
	}

	msg := routing.PlayingState {
		IsPaused: true,
	}

	exiting := false
	for {
		command := gamelogic.GetInput()
		if len(command) == 0 {
			continue
		} else {
			switch command[0] {
			case "pause":
				log.Printf("Sending pause")
				msg.IsPaused = true
				err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect,
					routing.PauseKey, msg)
				if err != nil {
					// fmt.Println("Error publishJSON:", err)
					log.Printf("Could not publish time: %v", err)
					// os.Exit(1)
				}

				log.Printf("Pause message sent")
			case "resume":
				log.Printf("Resuming")
				msg.IsPaused = false
				err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect,
					routing.PauseKey, msg)
				if err != nil {
					// fmt.Println("Error publishJSON:", err)
					log.Printf("Could not publish time: %v", err)
					// os.Exit(1)
				}

				log.Printf("Resume message sent")
			case "help":
				gamelogic.PrintServerHelp()
			case "quit":
				log.Printf("Exiting")
				exiting = true
			default:
				log.Printf("Unknown command %v", command[0])
			}
			if exiting {
				break
			}
		}
	}

	// Wait for ctrl+c
	/**
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<- signalChan
	fmt.Println("interrupt received")
	**/

}
