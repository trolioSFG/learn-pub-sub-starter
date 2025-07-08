package main

import (
	"fmt"
	"github.com/trolioSFG/learn-pub-sub-starter/internal/pubsub"
	"github.com/trolioSFG/learn-pub-sub-starter/internal/routing"
	"github.com/trolioSFG/learn-pub-sub-starter/internal/gamelogic"
	"os"
//	"os/signal"
	"log"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")
	url := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(url)
	if err != nil {
		fmt.Println("Error: %w", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("Connection success")

	publishCh, err := conn.Channel()
	if err != nil {
		log.Fatalf("Could not create channel: %v", err)
	}

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Printf("ClientWelcome error: %v", err)
		os.Exit(1)
	}

	state := gamelogic.NewGameState(username)

	/**
	NO NEED for DeclareAndBind, done in SubscribeJSON
	queueName := routing.PauseKey + "." + username
	_, q, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect,
		queueName, routing.PauseKey, pubsub.Transient)
	if err != nil {
		log.Printf("DeclareAndBind error: %v", err)
		os.Exit(1)
	}
	**/

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect,
		routing.PauseKey + "." + state.GetUsername(),
		routing.PauseKey, pubsub.Transient, handlerPause(state))

	if err != nil {
		log.Printf("Error subscribeJSON: %v", err)
	}

	/**
	queueName = "army_moves." + username
	chm, qm, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic,
		queueName, "army_moves.*", pubsub.Transient)
	if err != nil {
		log.Printf("Could not bind to army_moves: %v", err)
		os.Exit(1)
	}
	**/

	// Army moves
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic,
		"army_moves." + state.GetUsername(),
		"army_moves.*", pubsub.Transient, handlerMove(state, publishCh))
	if err != nil {
		fmt.Printf("Error subscribing to army_moves: %v", err)
	}

	// War
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic,
		"war",	
		routing.WarRecognitionsPrefix + ".*", pubsub.Durable,
		handlerConsumeWar(state))
	if err != nil {
		fmt.Printf("Error subscribing to war_recognitions: %v", err)
	}

	exiting := false
	for {
		cmd := gamelogic.GetInput()
		if len(cmd) == 0 {
			continue
		}

		switch cmd[0] {
		case "spawn":
			err = state.CommandSpawn(cmd)
			if err != nil {
				log.Printf("Spawn error: %v", err)
			}
		case "move":
			mv, err := state.CommandMove(cmd)
			if err != nil {
				log.Printf("Move error: %v", err)
			}
			err = pubsub.PublishJSON(publishCh, routing.ExchangePerilTopic,
				"army_moves." + mv.Player.Username, mv)
			if err != nil {
				fmt.Printf("Publish move error: %v", err)
				continue
			}
			fmt.Printf("Moved %d units to %s", len(mv.Units), mv.ToLocation)
		case "status":
			state.CommandStatus()
		case "spam":
			fmt.Printf("Spamming not allowed yet!")
		case "help":
			gamelogic.PrintClientHelp()
		case "quit":
			gamelogic.PrintQuit()
			exiting = true
		default:
			log.Printf("Unknown command: %v", cmd[0])
		}
		if exiting {
			break
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(r routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(r)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, publishCh *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {


		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			// return pubsub.Ack
			/** INNECESARIO, publishCh lo pasamos como argumento
			url := "amqp://guest:guest@localhost:5672/"
			conn, err := amqp.Dial(url)
			if err != nil {
				fmt.Println("Error: %w", err)
				os.Exit(1)
			}
			defer conn.Close()
			publishCh, err := conn.Channel()
			***/
			err := pubsub.PublishJSON(publishCh, routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix + "." + gs.GetUsername(),
				gamelogic.RecognitionOfWar{ Attacker: move.Player, Defender: gs.GetPlayerSnap(),})
			if err != nil {
				return pubsub.NackRequeue
			} else {
				return pubsub.Ack
			}
		default:
			return pubsub.NackDiscard
		}
	}
}


func handlerConsumeWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(row gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, _, _ := gs.HandleWar(row)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			fmt.Println("Error: unknown outcome of war")
			return pubsub.NackDiscard
		}
	}
}

