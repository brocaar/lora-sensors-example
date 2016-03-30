package main

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brocaar/loraserver"
	"github.com/codegangsta/cli"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/influxdata/influxdb/client/v2"
)

var influxClient client.Client

func run(c *cli.Context) {
	var err error

	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.String("mqtt-server"))
	opts.SetUsername(c.String("mqtt-username"))
	opts.SetPassword(c.String("mqtt-password"))
	opts.SetOnConnectHandler(onConnected)

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	// Make client
	influxClient, err = client.NewHTTPClient(client.HTTPConfig{
		Addr:     c.String("influx-url"),
		Username: c.String("influx-user"),
		Password: c.String("influx-password"),
	})
	if err != nil {
		log.Fatal(err)
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	log.Println(<-sigChan)
}

func onConnected(c mqtt.Client) {
	log.Println("connected to mqtt server")
	if token := c.Subscribe("application/0101010101010101/node/+/rx", 0, onData); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
}

func onData(c mqtt.Client, msg mqtt.Message) {
	var rxPL loraserver.RXPayload
	if err := json.Unmarshal(msg.Payload(), &rxPL); err != nil {
		log.Println(err)
		return
	}
	log.Printf("topic: %s, payload: %+v", msg.Topic(), rxPL)
	switch rxPL.FPort {
	case 1:
		handleAirQuality(rxPL)
	case 2:
		handleTemperature(rxPL)
	default:
		log.Printf("unknown FPort: %d", rxPL.FPort)
	}
}

func handleAirQuality(rxPL loraserver.RXPayload) {
	quality := binary.LittleEndian.Uint16(rxPL.Data)
	log.Printf("air-quality: %d", quality)

	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  "sensors",
		Precision: "s",
	})
	if err != nil {
		log.Println(err)
		return
	}
	tags := map[string]string{"devEUI": rxPL.DevEUI.String()}
	fields := map[string]interface{}{
		"quality": quality,
	}
	pt, err := client.NewPoint("air_quality", tags, fields, time.Now())
	if err != nil {
		log.Println(err)
		return
	}
	bp.AddPoint(pt)

	if err := influxClient.Write(bp); err != nil {
		log.Println(err)
	}
}

func handleTemperature(rxPL loraserver.RXPayload) {
	tempint := binary.LittleEndian.Uint32(rxPL.Data)
	tempFloat := math.Float32frombits(tempint)
	log.Println("temperature: %f", tempFloat)

	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  "sensors",
		Precision: "s",
	})
	if err != nil {
		log.Println(err)
		return
	}
	tags := map[string]string{"devEUI": rxPL.DevEUI.String()}
	fields := map[string]interface{}{
		"celcius": tempFloat,
	}
	pt, err := client.NewPoint("temperature", tags, fields, time.Now())
	if err != nil {
		log.Println(err)
		return
	}
	bp.AddPoint(pt)

	if err := influxClient.Write(bp); err != nil {
		log.Println(err)
	}
}

func main() {
	app := cli.NewApp()
	app.Usage = "handler for air-quality sensor payloads"
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "mqtt-server",
			Usage:  "MQTT server",
			Value:  "tcp://localhost:1883",
			EnvVar: "MQTT_SERVER",
		},
		cli.StringFlag{
			Name:   "mqtt-username",
			Usage:  "MQTT username",
			EnvVar: "MQTT_USERNAME",
		},
		cli.StringFlag{
			Name:   "mqtt-password",
			Usage:  "MQTT password",
			EnvVar: "MQTT_PASSWORD",
		},
		cli.StringFlag{
			Name:   "influx-url",
			Value:  "http://localhost:8086",
			Usage:  "InfluxDB URL",
			EnvVar: "INFLUX_URL",
		},
		cli.StringFlag{
			Name:   "influx-user",
			Usage:  "InfluxDB user",
			EnvVar: "INFLUX_USER",
		},
		cli.StringFlag{
			Name:   "influx-password",
			Usage:  "InfluxDB password",
			EnvVar: "INFLUX_PASSWORD",
		},
	}
	app.Run(os.Args)
}
