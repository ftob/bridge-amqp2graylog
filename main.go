package main

import (
	"github.com/streadway/amqp"
	"fmt"
	"log"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
	"io"
	"os"
	"time"
)


var (
	Conn *amqp.Connection
	Channel *amqp.Channel
	config *Config
	GraylogWriter *gelf.TCPWriter
)

type Config struct {
	GrayLogHost string
	GrayLogPort string
	RabbitHost string
	RabbitPort string
	RabbitUser string
	RabbitPassword string
	RabbitQueue string
	GrayLogDsn string
	RabbitDsn string
}

func (cnf *Config) MakeGrayLogDsn() {
	cnf.GrayLogDsn = fmt.Sprintf("%s:%s", cnf.GrayLogHost, cnf.GrayLogPort)
}

func (cnf *Config) MakeRabbitDsn() {
	cnf.RabbitDsn = fmt.Sprintf("amqp://%s:%s@%s:%s/", cnf.RabbitUser, cnf.RabbitPassword, cnf.RabbitHost, cnf.RabbitPort)
}

func failOnError(err error, msg string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}


func initConfig() {
	config = new(Config)
	config.GrayLogHost = os.Getenv("GRAYLOG_HOST")

	if len(config.GrayLogHost) == 0 {
		config.GrayLogHost = "graylog"
	}

	config.GrayLogPort = os.Getenv("GRAYLOG_PORT")

	if len(config.GrayLogPort) == 0 {
		config.GrayLogPort = "12201"
	}

	config.RabbitHost = os.Getenv("RABBIT_HOST")

	if len(config.RabbitHost) == 0 {
		config.RabbitHost = "rabbit"
	}

	config.RabbitPort = os.Getenv("RABBIT_PORT")

	if len(config.RabbitPort) == 0 {
		config.RabbitPort = "5672"
	}

	config.RabbitUser = os.Getenv("RABBIT_USER")

	if len(config.RabbitUser) == 0 {
		config.RabbitUser = "guest"
	}
	config.RabbitPassword = os.Getenv("RABBIT_PASSWORD")

	if len(config.RabbitPassword) == 0 {
		config.RabbitPassword = "guest"
	}

	config.RabbitQueue = os.Getenv("RABBIT_QUEUE")

	if len(config.RabbitQueue) == 0 {
		config.RabbitQueue = "logs"
	}

	config.MakeGrayLogDsn()
	config.MakeRabbitDsn()
}

func init()  {
	duration := time.Duration(10) * time.Second
	time.Sleep(duration)


	var err error

	initConfig()
	fmt.Printf("Connect GrayLog - %s\n", config.GrayLogDsn)
	GraylogWriter, err = gelf.NewTCPWriter(config.GrayLogDsn)
	failOnError(err, "Fail connection to GrayLog")

	fmt.Printf("Connect RabbitMQ - %s\n", config.RabbitDsn)
	Conn, err := amqp.Dial(config.RabbitDsn)
	failOnError(err, "Failed to connect to RabbitMQ")

	Channel, err = Conn.Channel()
	failOnError(err, "Failed to open a channel")
}


func main() {
	defer Conn.Close()
	defer Channel.Close()

	// log to both stderr and graylog2
	log.SetOutput(io.MultiWriter(os.Stderr, GraylogWriter))

	err := Channel.ExchangeDeclare(
		config.RabbitQueue,   // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	q, err := Channel.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when usused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = Channel.QueueBind(
		q.Name, // queue name
		"",     // routing key
		config.RabbitQueue, // exchange
		false,
		nil)

	failOnError(err, "Failed to bind a queue")


	msgs, err := Channel.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf(" [x] %s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for logs. To exit press CTRL+C")
	<-forever
}
