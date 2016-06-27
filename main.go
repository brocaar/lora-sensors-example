package main

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"

	"github.com/brocaar/loraserver/models"
	"github.com/codegangsta/cli"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	temp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lora_sensor_temperature_celsius",
		Help: "Current temperature in C",
	})

	airq = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lora_sensor_airquality",
		Help: "Current air quality",
	})
)

func init() {
	prometheus.MustRegister(temp)
	prometheus.MustRegister(airq)
}

func run(c *cli.Context) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.String("mqtt-server"))
	opts.SetUsername(c.String("mqtt-username"))
	opts.SetPassword(c.String("mqtt-password"))
	opts.SetOnConnectHandler(onConnected)

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(":8080", nil)
}

func onConnected(c mqtt.Client) {
	log.Println("connected to mqtt server")
	if token := c.Subscribe("application/0101010101010101/node/0202020202020202/rx", 0, onData); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
}

func onData(c mqtt.Client, msg mqtt.Message) {
	var rxPL models.RXPayload
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

func handleAirQuality(rxPL models.RXPayload) {
	quality := binary.LittleEndian.Uint16(rxPL.Data)
	log.Printf("air-quality: %d", quality)
	airq.Set(float64(quality))

}

func handleTemperature(rxPL models.RXPayload) {
	tempint := binary.LittleEndian.Uint32(rxPL.Data)
	tempFloat := math.Float32frombits(tempint)
	log.Printf("temperature: %f", tempFloat)
	temp.Set(float64(tempFloat))
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
	}
	app.Run(os.Args)
}
