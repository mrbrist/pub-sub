package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")

	gamelogic.PrintServerHelp()

	conStr := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(conStr)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	fmt.Println("Connection successful...")

	ch, err := connection.Channel()
	if err != nil {
		log.Fatal(err)
	}

	err = pubsub.SubscribeGob(connection, "peril_topic", "game_logs", "game_logs.*", pubsub.SimpleQueueDurable, handlerLogs())
	if err != nil {
		log.Fatal(err)
	}

	for {
		input := gamelogic.GetInput()
		if input == nil {
			continue
		}
		switch input[0] {
		case "pause":
			fmt.Println("Sending pause message...")
			pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: true,
			})

		case "resume":
			fmt.Println("Sending resume message...")
			pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: false,
			})

		case "quit":
			fmt.Println("Sending quit message...")
			return
		default:
			fmt.Println("Unknown command...")
		}
	}
}

func handlerLogs() func(routing.GameLog) pubsub.AckType {
	return func(gl routing.GameLog) pubsub.AckType {
		defer fmt.Print("> ")

		gamelogic.WriteLog(gl)

		return pubsub.Ack
	}
}
