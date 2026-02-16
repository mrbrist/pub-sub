package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

var globalCh *amqp.Channel

func main() {
	fmt.Println("Starting Peril client...")

	conStr := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(conStr)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	fmt.Println("Connection successful...")

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatal(err)
	}

	// _, _, err = pubsub.DeclareAndBind(connection, "peril_direct", fmt.Sprintf("pause.%s", username), "pause", pubsub.SimpleQueueTransient)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	state := gamelogic.NewGameState(username)

	pubsub.SubscribeJSON(connection, routing.ExchangePerilDirect, fmt.Sprintf("pause.%s", username), routing.PauseKey, pubsub.SimpleQueueTransient, handlerPause(state))

	pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username), "army_moves.*", pubsub.SimpleQueueTransient, handlerMove(state))

	pubsub.SubscribeJSON(connection, routing.ExchangePerilTopic, "war", "war.*", pubsub.SimpleQueueDurable, handlerWar(state))

	ch, err := connection.Channel()
	if err != nil {
		log.Fatal(err)
	}

	globalCh = ch

	for {
		input := gamelogic.GetInput()
		if input == nil {
			continue
		}
		switch input[0] {
		case "spawn":
			err := state.CommandSpawn(input)
			if err != nil {
				fmt.Println("Could not spawn")
				continue
			}
			fmt.Println("Spawn successful!")
		case "move":
			am, err := state.CommandMove(input)
			if err != nil {
				fmt.Println("Could not move")
				continue
			}
			pubsub.PublishJSON(ch, routing.ExchangePerilTopic, fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username), gamelogic.ArmyMove{
				Player:     am.Player,
				Units:      am.Units,
				ToLocation: am.ToLocation,
			})
			fmt.Println("Move published!")
		case "status":
			state.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Unknown command...")
		}
	}
}

func PublishGamelog(gl routing.GameLog) error {
	err := pubsub.PublishGob(globalCh, routing.ExchangePerilTopic, fmt.Sprintf("%s.%s", routing.GameLogSlug, gl.Username), gl)
	if err != nil {
		return err
	}
	return nil
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")

		gs.HandlePause(ps)

		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(am gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")

		switch gs.HandleMove(am) {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(globalCh, routing.ExchangePerilTopic, fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, gs.GetUsername()), gamelogic.RecognitionOfWar{
				Attacker: am.Player,
				Defender: gs.GetPlayerSnap(),
			})
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}

	}
}

func handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")

		out, winner, loser := gs.HandleWar(rw)

		switch out {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			err := PublishGamelog(routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
				Username:    gs.GetUsername(),
			})
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			err := PublishGamelog(routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
				Username:    gs.GetUsername(),
			})
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			err := PublishGamelog(routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser),
				Username:    gs.GetUsername(),
			})
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			fmt.Println("Unknown war outcome")
			return pubsub.NackDiscard
		}
	}
}
